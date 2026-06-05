package handlers

import (
	"context"
	"sync"

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
type outFile struct {
	srcBase string
	preset  string
	format  processor.Format
	data    []byte
}

// srcFile is one uploaded image held in memory while its job runs.
type srcFile struct {
	base string
	data []byte
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

	mu       sync.Mutex
	total    int
	done     int
	status   string // processing | complete | error
	finished bool
	events   []progressEvent      // append-only history for replay
	outputs  []outFile            // successful outputs for the ZIP
	subs     []chan progressEvent // active SSE subscribers
}

// Store is the in-memory job registry. No Redis, no disk: jobs live until their
// ZIP is downloaded (or until the process exits).
type Store struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

// NewStore returns an empty job store.
func NewStore() *Store {
	return &Store{jobs: make(map[string]*Job)}
}

// Create registers a new job with the given total work-unit count and returns
// it. total is len(files) * len(presets).
func (s *Store) Create(total int) *Job {
	j := &Job{
		ID:     uuid.NewString(),
		store:  s,
		total:  total,
		status: "processing",
	}
	s.mu.Lock()
	s.jobs[j.ID] = j
	s.mu.Unlock()
	return j
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

// runJob processes every file through every preset, streaming a progress event
// per completed (file, preset) unit, then finishes the job. It runs in its own
// goroutine off the upload request and therefore uses context.Background(): the
// request context is canceled the moment the upload handler returns.
func runJob(job *Job, files []srcFile, presets []processor.Preset) {
	for _, f := range files {
		f := f
		_, err := processor.ProcessStream(context.Background(), f.data, presets,
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
					})
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
	job.Finish("/download/" + job.ID)
}
