//go:build !vips

// This fallback is compiled when the `vips` build tag is absent (every local
// build/test, since libvips dev headers only exist in the Docker builder stage).
// It provides the same exported surface as the real pipeline so the tag-free
// main.go links in both modes. Startup is a no-op so the local dev server still
// boots and serves /health and the SPA; only image processing is unavailable.

package processor

import (
	"context"
)

// ErrVipsNotBuilt is declared in preset.go (tag-free) so it exists in both the
// vips and non-vips builds.

// Startup is a no-op in non-vips builds. The workers argument is accepted to
// match the real signature but ignored, since no processing runs.
func Startup(workers int) error { return nil }

// Shutdown is a no-op in non-vips builds.
func Shutdown() {}

// Process reports that libvips support was not compiled in.
func Process(ctx context.Context, buf []byte, presets []Preset) ([]Result, error) {
	return nil, ErrVipsNotBuilt
}

// ProcessStream reports that libvips support was not compiled in. The onResult
// callback is never invoked in this build.
func ProcessStream(ctx context.Context, buf []byte, presets []Preset, onResult ResultFunc) ([]Result, error) {
	return nil, ErrVipsNotBuilt
}

// ProcessBundle reports (via Result.Err) that libvips support was not compiled
// in. The error is carried in the Result, matching the real implementation's
// failure path so runJob's bundle phase handles both modes identically.
func ProcessBundle(ctx context.Context, bufs [][]byte, p Preset) Result {
	return Result{Preset: p, Err: ErrVipsNotBuilt}
}
