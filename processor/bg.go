package processor

// This file is intentionally tag-free, mirroring preset.go: it imports neither
// govips nor onnxruntime, so it compiles in every build mode. It provides the
// dispatch seam between the (vips-tagged) image pipeline and the optional
// (onnx-tagged) background-removal implementation.
//
// The real implementation lives in processor/bg_onnx.go behind the `onnx` build
// tag; its init() assigns removeBackgroundFn. When the binary is built without
// `onnx`, removeBackgroundFn stays nil and removeBackground returns
// ErrONNXNotBuilt via Result.Err — the same graceful per-preset degradation the
// `vips` stub uses, but decoupled from the vips build so the two tags compose
// freely.

import "errors"

// ErrONNXNotBuilt is carried in Result.Err when a KindBackgroundRemove preset is
// run on a binary built without the `onnx` tag (every local build, since the
// onnxruntime shared library is only vendored into the Docker image). Declared
// here (tag-free) so it exists in every build, alongside ErrVipsNotBuilt.
var ErrONNXNotBuilt = errors.New("processor: built without 'onnx' tag; background removal unavailable")

// removeBackgroundFn is the registered background-removal implementation. It is
// nil unless the `onnx`-tagged bg_onnx.go was compiled in (its init() sets it).
var removeBackgroundFn func(buf []byte, p Preset) Result

// bgModelPath and bgLibPath hold the filesystem paths to the U²-Netp model and
// the onnxruntime shared library. They are set by ConfigureBackground (called
// from main at startup) and read by the onnx implementation on first use. Kept
// here, tag-free, so main.go can pass config in without importing onnxruntime.
var (
	bgModelPath string
	bgLibPath   string
)

// ConfigureBackground records the model and onnxruntime library paths for the
// background-removal pipeline. It is a no-op effect in non-onnx builds (the
// values are simply never read). Call once at startup, before any job runs.
func ConfigureBackground(modelPath, libPath string) {
	bgModelPath = modelPath
	bgLibPath = libPath
}

// removeBackground runs the background-removal pipeline for a KindBackgroundRemove
// preset, or reports ErrONNXNotBuilt (via Result.Err) when ONNX support was not
// compiled in. The image pipeline (processImage) dispatches here.
func removeBackground(buf []byte, p Preset) Result {
	if removeBackgroundFn == nil {
		return Result{Preset: p, Err: ErrONNXNotBuilt}
	}
	return removeBackgroundFn(buf, p)
}
