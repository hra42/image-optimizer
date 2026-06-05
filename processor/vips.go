//go:build vips

// This file is only compiled with the `vips` build tag, which requires libvips
// dev headers (present in the Docker builder stage). It anchors the govips/v2
// dependency and will hold the real libvips startup config in HRA-163. Local
// `go build ./...` skips it so the toolchain works without libvips installed.

package processor

import "github.com/davidbyttow/govips/v2/vips"

// Startup initializes libvips. The full config (concurrency, cache disabling,
// worker semaphore) is implemented in HRA-163.
func Startup() {
	vips.Startup(nil)
}

// Shutdown tears down libvips.
func Shutdown() {
	vips.Shutdown()
}
