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

// initSemaphores sizes the worker pool. n is the configured worker count
// (WORKER_COUNT); a non-positive value falls back to runtime.NumCPU().
func initSemaphores(n int) {
	if n < 1 {
		n = runtime.NumCPU()
	}
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
	return ProcessStream(ctx, buf, presets, nil)
}

// ProcessStream is Process with a per-preset callback. onResult (may be nil) is
// invoked as each preset finishes — see ResultFunc for the concurrency contract
// — which lets callers stream progress in real time instead of waiting for the
// whole batch. The returned slice and error semantics match Process.
func ProcessStream(ctx context.Context, buf []byte, presets []Preset, onResult ResultFunc) ([]Result, error) {
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

			r := processImage(buf, p)
			results[i] = r
			if onResult != nil {
				onResult(i, r)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}
	return results, nil
}

// ProcessBundle runs one bundle preset (KindDocumentPDF) over ALL uploaded
// buffers at once, producing a single Result whose Files holds one named PDF.
// bufs are in upload order; page order follows. Per-page renders go through the
// same worker admission as ProcessStream so a large carousel can't exceed the
// libvips concurrency cap; pages are rendered sequentially (ordered, bounded
// memory — carousels are small). Failures land in Result.Err.
func ProcessBundle(ctx context.Context, bufs [][]byte, p Preset) Result {
	res := Result{Preset: p}

	pages := make([]pdfPage, 0, len(bufs))
	for _, buf := range bufs {
		// Check cancellation first: a select with both a ready semaphore and a
		// done context picks at random, so an already-cancelled context could
		// otherwise slip through the admission selects below.
		if err := ctx.Err(); err != nil {
			res.Err = err
			return res
		}

		// Admission to the queue (back-pressure), then a worker slot.
		select {
		case queueSem <- struct{}{}:
		case <-ctx.Done():
			res.Err = ctx.Err()
			return res
		}
		select {
		case activeSem <- struct{}{}:
		case <-ctx.Done():
			<-queueSem
			res.Err = ctx.Err()
			return res
		}

		pg, err := renderDocumentPage(buf, p)
		<-activeSem
		<-queueSem
		if err != nil {
			res.Err = err
			return res
		}
		pages = append(pages, pg)
	}

	data, err := buildDocumentPDF(pages)
	if err != nil {
		res.Err = err
		return res
	}
	res.Files = []OutputFile{{Name: p.Name + ".pdf", Data: data}}
	return res
}
