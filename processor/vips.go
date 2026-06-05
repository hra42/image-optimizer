//go:build vips

// This file is only compiled with the `vips` build tag, which requires libvips
// dev headers (present in the Docker builder stage). Local `go build ./...`
// skips it so the toolchain works without libvips installed; the non-vips
// fallback in process_stub.go supplies matching no-op symbols.

package processor

import "github.com/davidbyttow/govips/v2/vips"

// Startup initializes libvips with imgproxy-inspired tuning and primes the
// worker semaphores. Concurrency is pinned to 1 so Go goroutines own
// parallelism, and the operation cache is fully disabled — this both prevents
// the SIGSEGV seen on Alpine/Musl and keeps memory flat under load (no cache
// accumulation across requests).
func Startup() error {
	initSemaphores()
	return vips.Startup(&vips.Config{
		ConcurrencyLevel: 1, // vips_concurrency_set(1) — Go handles parallelism
		MaxCacheFiles:    0,
		MaxCacheMem:      0,
		MaxCacheSize:     0,
	})
}

// Shutdown tears down libvips.
func Shutdown() {
	vips.Shutdown()
}
