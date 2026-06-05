//go:build !vips

// This fallback is compiled when the `vips` build tag is absent (every local
// build/test, since libvips dev headers only exist in the Docker builder stage).
// It provides the same exported surface as the real pipeline so the tag-free
// main.go links in both modes. Startup is a no-op so the local dev server still
// boots and serves /health and the SPA; only image processing is unavailable.

package processor

import (
	"context"
	"errors"
)

// ErrVipsNotBuilt is returned by Process when the binary was built without the
// `vips` tag, i.e. without libvips support compiled in.
var ErrVipsNotBuilt = errors.New("processor: built without 'vips' tag; libvips unavailable")

// Startup is a no-op in non-vips builds.
func Startup() error { return nil }

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
