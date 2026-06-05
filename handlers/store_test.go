package handlers

import (
	"context"
	"sync"
	"testing"
	"time"
)

// drain reads every remaining event from a closed/closing channel.
func drain(ch <-chan progressEvent) []progressEvent {
	var out []progressEvent
	for ev := range ch {
		out = append(out, ev)
	}
	return out
}

// TestSubscribeReplaysHistory: events emitted before a subscriber attaches must
// be replayed to it — this is the upload→subscribe race the store must close.
func TestSubscribeReplaysHistory(t *testing.T) {
	s := NewStore()
	job := s.Create(3)

	// Three events fire before anyone subscribes.
	for i := 0; i < 3; i++ {
		job.mu.Lock()
		job.publishLocked(progressEvent{Job: job.ID, Pct: (i + 1) * 25, Status: "processing"})
		job.mu.Unlock()
	}

	history, ch, done, ok := s.Subscribe(job.ID)
	if !ok {
		t.Fatal("Subscribe returned ok=false for an existing job")
	}
	if done {
		t.Fatal("Subscribe reported done before the job finished")
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 replayed events, got %d", len(history))
	}
	if ch == nil {
		t.Fatal("expected a live channel for an unfinished job")
	}
}

// TestSubscribeAfterFinishReplaysTerminal: subscribing after the job completes
// returns the full history (including the terminal event) and done=true.
func TestSubscribeAfterFinishReplaysTerminal(t *testing.T) {
	s := NewStore()
	job := s.Create(1)

	job.mu.Lock()
	job.publishLocked(progressEvent{Job: job.ID, Pct: 100, Status: "processing"})
	job.mu.Unlock()
	job.Finish("/download/" + job.ID)

	history, ch, done, ok := s.Subscribe(job.ID)
	if !ok {
		t.Fatal("Subscribe returned ok=false for an existing job")
	}
	if !done {
		t.Fatal("expected done=true for a finished job")
	}
	if ch != nil {
		t.Fatal("expected nil channel for a finished job")
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 events (processing + complete), got %d", len(history))
	}
	last := history[len(history)-1]
	if last.Status != "complete" || last.DownloadURL != "/download/"+job.ID {
		t.Fatalf("terminal event wrong: %+v", last)
	}
}

// TestSubscribeUnknownJob: an unknown id yields ok=false.
func TestSubscribeUnknownJob(t *testing.T) {
	s := NewStore()
	if _, _, _, ok := s.Subscribe("does-not-exist"); ok {
		t.Fatal("expected ok=false for an unknown job")
	}
}

// TestLiveDeliveryAndFinishClosesChannel: an attached subscriber receives live
// events, and Finish closes its channel so the SSE loop can exit.
func TestLiveDeliveryAndFinishClosesChannel(t *testing.T) {
	s := NewStore()
	job := s.Create(2)

	_, ch, _, ok := s.Subscribe(job.ID)
	if !ok || ch == nil {
		t.Fatal("Subscribe failed to return a live channel")
	}

	job.mu.Lock()
	job.publishLocked(progressEvent{Job: job.ID, Pct: 50, Status: "processing"})
	job.mu.Unlock()
	job.Finish("/download/" + job.ID)

	got := drain(ch) // ranges until Finish closed the channel
	if len(got) != 2 {
		t.Fatalf("expected 2 live events (processing + complete), got %d", len(got))
	}
	if got[len(got)-1].Status != "complete" {
		t.Fatalf("expected last event complete, got %+v", got[len(got)-1])
	}
}

// TestConcurrentPublishAndSubscribe: with the race detector on, concurrent
// publishes and a subscription must not race and must lose no event. Every
// emitted event lands either in the replayed history or on the live channel.
func TestConcurrentPublishAndSubscribe(t *testing.T) {
	const n = 50
	s := NewStore()
	job := s.Create(n)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			job.mu.Lock()
			job.publishLocked(progressEvent{Job: job.ID, Pct: i, Status: "processing"})
			job.mu.Unlock()
		}
		job.Finish("/download/" + job.ID)
	}()

	history, ch, done, ok := s.Subscribe(job.ID)
	if !ok {
		t.Fatal("Subscribe returned ok=false")
	}

	total := len(history)
	if !done && ch != nil {
		total += len(drain(ch))
	}
	wg.Wait()

	// n processing events + 1 terminal complete event = n+1 total, with none lost.
	if total != n+1 {
		t.Fatalf("expected %d events across history+channel, got %d", n+1, total)
	}
}

// TestDeleteRemovesJob: after Delete, the job is gone (download cleanup).
func TestDeleteRemovesJob(t *testing.T) {
	s := NewStore()
	job := s.Create(1)
	s.Delete(job.ID)
	if _, ok := s.Get(job.ID); ok {
		t.Fatal("expected job to be removed after Delete")
	}
}

// TestReapExpiredRemovesOldKeepsFresh: a job older than the TTL is reaped while
// a fresh one survives. The clock is injected via s.now so no sleeping is needed.
func TestReapExpiredRemovesOldKeepsFresh(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)

	s := NewStore()
	s.now = func() time.Time { return base }
	old := s.Create(1) // created "now"

	// Advance the clock 11 minutes; a new job is created at the later time.
	s.now = func() time.Time { return base.Add(11 * time.Minute) }
	fresh := s.Create(1)

	s.reapExpired(10 * time.Minute)

	if _, ok := s.Get(old.ID); ok {
		t.Error("expected job older than TTL to be reaped")
	}
	if _, ok := s.Get(fresh.ID); !ok {
		t.Error("expected fresh job to survive reaping")
	}
}

// TestReapExpiredClosesSubscribers: reaping a job that still has a live SSE
// subscriber must close its channel so the SSE handler returns rather than leak.
func TestReapExpiredClosesSubscribers(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)

	s := NewStore()
	s.now = func() time.Time { return base }
	job := s.Create(1)

	_, ch, _, ok := s.Subscribe(job.ID)
	if !ok || ch == nil {
		t.Fatal("Subscribe failed to return a live channel")
	}

	s.now = func() time.Time { return base.Add(time.Hour) }
	s.reapExpired(10 * time.Minute)

	// Channel must be closed (drain returns once the range ends).
	done := make(chan struct{})
	go func() { drain(ch); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("subscriber channel was not closed by the reaper")
	}
}

// TestStartReaperStopsOnContextCancel: the reaper goroutine exits when its
// context is canceled, so graceful shutdown does not leak it.
func TestStartReaperStopsOnContextCancel(t *testing.T) {
	s := NewStore()
	ctx, cancel := context.WithCancel(context.Background())
	s.StartReaper(ctx, time.Minute)
	cancel()
	// Nothing to assert beyond not hanging/panicking; the goroutine returns on
	// ctx.Done(). A short wait gives the race detector a window to flag misuse.
	time.Sleep(10 * time.Millisecond)
}
