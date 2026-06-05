//go:build vips

package processor

import (
	"context"
	"runtime"

	"golang.org/x/sync/errgroup"
)

// Two semaphores implement the imgproxy worker pattern:
//
//   - activeSem caps the number of libvips pipelines running at once
//     (runtime.NumCPU()). This is the real concurrency limit that keeps memory
//     flat — only N pipelines hold image buffers simultaneously.
//   - queueSem is a wider buffer (2×N) so a burst of presets/requests queues and
//     applies back-pressure instead of spawning every pipeline up front.
//
// They are initialized in Startup before any Process call.
var (
	queueSem  chan struct{}
	activeSem chan struct{}
)

func initSemaphores() {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	activeSem = make(chan struct{}, n)
	queueSem = make(chan struct{}, n*2)
}

// Process runs every preset over the source buffer concurrently and returns one
// Result per preset, in the same order as the input slice.
//
// buf is read-only and safely shared across goroutines. Per-preset failures are
// captured in Result.Err so one bad preset does not abort the rest; the returned
// error is reserved for context cancellation.
func Process(ctx context.Context, buf []byte, presets []Preset) ([]Result, error) {
	results := make([]Result, len(presets))

	g, gctx := errgroup.WithContext(ctx)
	for i, p := range presets {
		i, p := i, p
		g.Go(func() error {
			// Admission to the queue (back-pressure), then a worker slot.
			select {
			case queueSem <- struct{}{}:
			case <-gctx.Done():
				return gctx.Err()
			}
			defer func() { <-queueSem }()

			select {
			case activeSem <- struct{}{}:
			case <-gctx.Done():
				return gctx.Err()
			}
			defer func() { <-activeSem }()

			results[i] = processImage(buf, p)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}
	return results, nil
}
