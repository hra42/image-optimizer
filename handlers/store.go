package handlers

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/hra42/image-optimizer/processor"
)

// progressEvent is the SSE wire schema. The JSON tags are the exact field names
// the Svelte frontend consumes; omitempty keeps "processing" events free of the
// downloadUrl field and the final "complete" event free of preset/pct noise.
type progressEvent struct {
	Job         string `json:"job"`
	Preset      string `json:"preset,omitempty"`
	Pct         int    `json:"pct"`
	Status      string `json:"status"` // processing | complete | error
	DownloadURL string `json:"downloadUrl,omitempty"`
}

// outFile is one finished (file, preset) output destined for the ZIP.
//
// A normal image preset sets data + format and leaves pack nil: it becomes one
// ZIP entry named "<preset>.<ext>". A pack preset (e.g. favicon) sets pack to
// its member files and leaves data nil: each member becomes an entry under a
// "<preset>/" folder in the ZIP.
type outFile struct {
	srcBase string
	preset  string
	format  processor.Format
	data    []byte
	pack    []processor.OutputFile
	// bundle marks an all-files-in-one output (a document PDF). Its pack member
	// name is the full filename and it is written at the ZIP root — never
	// namespaced by source — so srcBase is left empty.
	bundle bool
}

// srcFile is one uploaded image held in memory while its job runs. focal is the
// optional per-file crop anchor (from the upload's "focals" field); when unset,
// fixed-aspect presets fall back to the default attention crop.
type srcFile struct {
	base  string
	data  []byte
	focal processor.FocalPoint
}

// Job tracks one upload's lifecycle. Progress is reported as completed work
// units out of total, where a unit is one (file, preset) pair.
//
// Concurrency model: every field is guarded by mu. The append-only events slice
// plus the subscriber channels are what make the upload→subscribe race safe — a
// subscriber that attaches late replays events; one that attaches early streams
// them live. See Store.Subscribe.
type Job struct {
	ID    string
	store *Store

	// createdAt is when the job was registered; the reaper uses it for TTL.
	// It is set once at Create and never mutated, so it needs no lock.
	createdAt time.Time

	mu        sync.Mutex
	total     int
	done      int
	status    string // processing | complete | error
	finished  bool
	downloads int                  // count of completed downloads (see MarkDownloaded)
	events    []progressEvent      // append-only history for replay
	outputs   []outFile            // successful outputs for the ZIP
	subs      []chan progressEvent // active SSE subscribers
}

// Store is the in-memory job registry. No Redis, no disk: jobs live until their
// ZIP is downloaded, until the reaper frees them after the TTL, or until the
// process exits.
type Store struct {
	mu   sync.Mutex
	jobs map[string]*Job

	// now is the clock used for createdAt; tests override it to control TTL.
	now func() time.Time

	// wg tracks in-flight job goroutines so graceful shutdown can drain them.
	wg sync.WaitGroup
}

// NewStore returns an empty job store.
func NewStore() *Store {
	return &Store{
		jobs: make(map[string]*Job),
		now:  time.Now,
	}
}

// Create registers a new job with the given total work-unit count and returns
// it. total is len(files) * len(presets).
func (s *Store) Create(total int) *Job {
	j := &Job{
		ID:        uuid.NewString(),
		store:     s,
		createdAt: s.now(),
		total:     total,
		status:    "processing",
	}
	s.mu.Lock()
	s.jobs[j.ID] = j
	s.mu.Unlock()
	return j
}

// Go runs fn in a tracked goroutine so Wait (used by graceful shutdown) can
// block until every in-flight job has finished processing.
func (s *Store) Go(fn func()) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		fn()
	}()
}

// Wait blocks until all goroutines started via Go have returned.
func (s *Store) Wait() {
	s.wg.Wait()
}

// Get returns the job with the given ID.
func (s *Store) Get(id string) (*Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	return j, ok
}

// Delete removes a job from the store. Called after its ZIP has been streamed.
func (s *Store) Delete(id string) {
	s.mu.Lock()
	delete(s.jobs, id)
	s.mu.Unlock()
}

// StartReaper launches a background goroutine that periodically frees jobs whose
// state has outlived ttl, bounding memory for jobs that are never downloaded.
// All job state (including output bytes) is in memory, so dropping the map entry
// is enough for GC to reclaim it — there are no temp files to unlink. The
// goroutine exits when ctx is canceled (graceful shutdown). A non-positive ttl
// disables reaping.
func (s *Store) StartReaper(ctx context.Context, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	// Sweep often enough to honor the TTL without busy-looping: at most once a
	// minute, or every ttl for very short TTLs (tests, tight deploys).
	interval := ttl
	if interval > time.Minute {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.reapExpired(ttl)
			}
		}
	}()
}

// reapExpired deletes every job older than ttl. Reaped jobs still holding live
// SSE subscribers are failed first so their subscriber channels close and the
// SSE handlers return rather than leaking.
func (s *Store) reapExpired(ttl time.Duration) {
	cutoff := s.now().Add(-ttl)

	s.mu.Lock()
	var expired []*Job
	for id, j := range s.jobs {
		if j.createdAt.Before(cutoff) {
			expired = append(expired, j)
			delete(s.jobs, id)
		}
	}
	s.mu.Unlock()

	for _, j := range expired {
		// Close any still-open subscriber channels (no-op if already finished).
		j.Fail()
		log.Printf("reaper: freed job %s (age > %s)", j.ID, ttl)
	}
}

// Subscribe attaches an SSE consumer to a job. It returns a snapshot of every
// event emitted so far (history) plus a channel of future events. If the job has
// already finished, done is true and ch is nil — the caller replays history and
// stops. ok is false when no such job exists.
//
// The history snapshot and channel registration happen under a single lock, so
// any publish is observed exactly once: either it is already in history, or it
// arrives on the channel after registration — never both, never neither. This
// closes the race where the client opens its EventSource only after the upload
// response (and some events) have already gone out.
func (s *Store) Subscribe(id string) (history []progressEvent, ch <-chan progressEvent, done, ok bool) {
	j, found := s.Get(id)
	if !found {
		return nil, nil, false, false
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	history = make([]progressEvent, len(j.events))
	copy(history, j.events)

	if j.finished {
		return history, nil, true, true
	}

	// Buffer the channel for every event the job can still emit (remaining work
	// units + the terminal event) so a correct subscriber never blocks publish.
	bufN := j.total - j.done + 1
	if bufN < 1 {
		bufN = 1
	}
	c := make(chan progressEvent, bufN)
	j.subs = append(j.subs, c)
	return history, c, false, true
}

// Unsubscribe detaches an SSE consumer (handler returned or client disconnected)
// so later publishes don't target a dead reader. The channel is not closed here;
// only Finish/Fail close subscriber channels.
func (s *Store) Unsubscribe(id string, ch <-chan progressEvent) {
	j, ok := s.Get(id)
	if !ok {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for i, c := range j.subs {
		if c == ch {
			j.subs = append(j.subs[:i], j.subs[i+1:]...)
			return
		}
	}
}

// publishLocked appends an event to the replay history and fans it out to every
// active subscriber. The caller must hold j.mu. Sends are non-blocking as a
// safety net; subscriber channels are sized so they never actually drop.
func (j *Job) publishLocked(ev progressEvent) {
	j.events = append(j.events, ev)
	for _, c := range j.subs {
		select {
		case c <- ev:
		default:
		}
	}
}

// Finish marks the job complete, publishes the terminal event carrying the
// download URL, and closes every subscriber channel so SSE handlers drain and
// return.
func (j *Job) Finish(downloadURL string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.finished {
		return
	}
	j.finished = true
	j.status = "complete"
	j.publishLocked(progressEvent{Job: j.ID, Status: "complete", DownloadURL: downloadURL})
	j.closeSubsLocked()
}

// Fail marks the job errored, publishes a terminal error event, and closes
// subscriber channels. Used when processing could not run (e.g. libvips absent).
func (j *Job) Fail() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.finished {
		return
	}
	j.finished = true
	j.status = "error"
	j.publishLocked(progressEvent{Job: j.ID, Status: "error"})
	j.closeSubsLocked()
}

// closeSubsLocked closes and clears all subscriber channels. Caller holds j.mu.
func (j *Job) closeSubsLocked() {
	for _, c := range j.subs {
		close(c)
	}
	j.subs = nil
}

// MarkDownloaded records one completed download and returns the new running
// count. The download handler keeps a job alive through the first download (so an
// auto-download leaves the manual button working as a fallback) and frees it on
// the second — the user has clearly got the file by then. The TTL reaper is the
// backstop if a second download never comes.
func (j *Job) MarkDownloaded() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.downloads++
	return j.downloads
}

// Outputs returns the successful outputs collected so far.
func (j *Job) Outputs() []outFile {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]outFile, len(j.outputs))
	copy(out, j.outputs)
	return out
}

// Finished reports whether the job has reached a terminal state.
func (j *Job) Finished() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.finished
}

// runJob processes a job in two phases: a per-file phase that runs each file
// through every per-image preset (one progress unit per (file, preset) pair),
// then a bundle phase that runs each bundle preset (e.g. a document PDF) once
// over ALL files (one progress unit per bundle preset). It runs in its own
// goroutine off the upload request and therefore uses context.Background(): the
// request context is canceled the moment the upload handler returns.
func runJob(job *Job, files []srcFile, imagePresets, bundlePresets []processor.Preset) {
	// --- Per-file phase: each file through every per-image preset. ---
	for _, f := range files {
		f := f
		// The focal point is a property of the source file, but processImage only
		// sees a Preset. Stamp the file's focal onto a per-file copy of the preset
		// slice so the crop path can read it without changing any signatures. An
		// unset focal is the zero value, so files the user didn't adjust keep the
		// default attention crop.
		filePresets := make([]processor.Preset, len(imagePresets))
		copy(filePresets, imagePresets)
		for i := range filePresets {
			filePresets[i].Focal = f.focal
		}
		_, err := processor.ProcessStream(context.Background(), f.data, filePresets,
			func(_ int, r processor.Result) {
				job.mu.Lock()
				job.done++
				pct := 100
				if job.total > 0 {
					pct = job.done * 100 / job.total
				}
				if r.Err == nil {
					job.outputs = append(job.outputs, outFile{
						srcBase: f.base,
						preset:  r.Preset.Name,
						format:  r.Preset.Format,
						data:    r.Data,
						pack:    r.Files,
					})
				} else {
					// A single failed preset is non-fatal by design (the other
					// presets/files still produce output), but log it so the
					// silently-missing ZIP entry is at least diagnosable.
					log.Printf("job %s: preset %q failed for %q: %v", job.ID, r.Preset.Name, f.base, r.Err)
				}
				job.publishLocked(progressEvent{
					Job:    job.ID,
					Preset: r.Preset.Name,
					Pct:    pct,
					Status: "processing",
				})
				job.mu.Unlock()
			})
		if err != nil {
			// Processing unavailable (e.g. built without the vips tag) or the
			// context was canceled — surface a terminal error and stop.
			job.Fail()
			return
		}
	}

	// --- Bundle phase: each bundle preset consumes ALL files once, in upload
	// order, and emits one top-level output (e.g. a multi-page PDF). ---
	if len(bundlePresets) > 0 {
		bufs := make([][]byte, len(files))
		for i, f := range files {
			bufs[i] = f.data // files is already in upload order; page order follows
		}
		for _, p := range bundlePresets {
			r := processor.ProcessBundle(context.Background(), bufs, p)
			if r.Err == processor.ErrVipsNotBuilt {
				job.Fail()
				return
			}
			job.mu.Lock()
			job.done++
			pct := 100
			if job.total > 0 {
				pct = job.done * 100 / job.total
			}
			if r.Err == nil {
				job.outputs = append(job.outputs, outFile{
					preset: r.Preset.Name,
					bundle: true,
					pack:   r.Files,
				})
			} else {
				// A failed bundle (e.g. a corrupt page or PDF assembly error) is
				// non-fatal so other selected presets still complete, but the
				// requested PDF will be absent from the ZIP — log it rather than
				// dropping it silently.
				log.Printf("job %s: bundle preset %q failed: %v", job.ID, r.Preset.Name, r.Err)
			}
			job.publishLocked(progressEvent{
				Job:    job.ID,
				Preset: r.Preset.Name,
				Pct:    pct,
				Status: "processing",
			})
			job.mu.Unlock()
		}
	}

	job.Finish("/download/" + job.ID)
}
