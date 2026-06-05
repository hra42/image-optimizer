//go:build vips

package processor

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
)

func TestMain(m *testing.M) {
	if err := Startup(0); err != nil {
		panic(err)
	}
	defer Shutdown()
	m.Run()
}

// makeSourcePNG builds a deterministic w×h RGB PNG in memory so the pipeline has
// real content to resize. A diagonal gradient avoids a flat image that some
// encoders special-case.
func makeSourcePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: uint8((x + y) % 256), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode source png: %v", err)
	}
	return buf.Bytes()
}

func TestProcessAllPresets(t *testing.T) {
	const srcW, srcH = 2000, 1500
	src := makeSourcePNG(t, srcW, srcH)

	presets := AllPresets()
	results, err := Process(context.Background(), src, presets)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if len(results) != len(presets) {
		t.Fatalf("got %d results, want %d", len(results), len(presets))
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("preset %q failed: %v", r.Preset.Name, r.Err)
			continue
		}

		// Pack presets return a set of named files instead of a single image.
		if r.Preset.Kind == KindFaviconPack {
			assertFaviconPack(t, r)
			continue
		}

		if len(r.Data) == 0 {
			t.Errorf("preset %q produced empty output", r.Preset.Name)
			continue
		}

		// Determine the expected output dimensions: fixed for cropping presets,
		// source dimensions for the website_* presets.
		wantW, wantH := r.Preset.Width, r.Preset.Height
		if !r.Preset.Resizes() {
			wantW, wantH = srcW, srcH
		}
		if r.Width != wantW || r.Height != wantH {
			t.Errorf("preset %q reported %dx%d, want %dx%d", r.Preset.Name, r.Width, r.Height, wantW, wantH)
		}

		// Decode the actual output and assert the real pixel dimensions match
		// exactly — this is the core acceptance criterion.
		out, err := vips.NewImageFromBuffer(r.Data)
		if err != nil {
			t.Errorf("preset %q output failed to decode: %v", r.Preset.Name, err)
			continue
		}
		if out.Width() != wantW || out.Height() != wantH {
			t.Errorf("preset %q decoded %dx%d, want %dx%d", r.Preset.Name, out.Width(), out.Height(), wantW, wantH)
		}
		out.Close()
	}
}

// assertFaviconPack checks the favicon pack has every expected member, that the
// PNGs decode to their named square dimensions, and that favicon.ico is present
// and non-empty.
func assertFaviconPack(t *testing.T, r Result) {
	t.Helper()

	if len(r.Files) == 0 {
		t.Fatalf("favicon pack produced no files")
	}

	byName := make(map[string][]byte, len(r.Files))
	for _, f := range r.Files {
		if len(f.Data) == 0 {
			t.Errorf("favicon member %q is empty", f.Name)
		}
		byName[f.Name] = f.Data
	}

	// Every named PNG size must be present and decode to its square dimension.
	wantPNG := map[string]int{
		"favicon-16x16.png":          16,
		"favicon-32x32.png":          32,
		"favicon-48x48.png":          48,
		"apple-touch-icon.png":       180,
		"android-chrome-192x192.png": 192,
		"android-chrome-512x512.png": 512,
	}
	for name, px := range wantPNG {
		data, ok := byName[name]
		if !ok {
			t.Errorf("favicon pack missing %q", name)
			continue
		}
		img, err := vips.NewImageFromBuffer(data)
		if err != nil {
			t.Errorf("favicon member %q failed to decode: %v", name, err)
			continue
		}
		if img.Width() != px || img.Height() != px {
			t.Errorf("favicon member %q is %dx%d, want %dx%d", name, img.Width(), img.Height(), px, px)
		}
		img.Close()
	}

	for _, name := range []string{"favicon.ico", "site.webmanifest", "README.txt"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("favicon pack missing %q", name)
		}
	}
}

// svgSource is a minimal vector document whose intrinsic size is a small
// 64×64. Without the density bump, the "keep original size" presets would
// rasterize it at ~64px; with svgRasterDensity (4×) they should land far larger.
const svgSource = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
  <rect width="64" height="64" fill="#1e1e2e"/>
  <circle cx="32" cy="32" r="24" fill="#89b4fa"/>
</svg>`

// TestProcessSVGInput verifies SVG input flows through every preset: resize
// presets render the vector straight to their exact target dimensions, and the
// "keep original size" presets rasterize at the bumped density so the output is
// a usable size rather than the SVG's tiny intrinsic dimensions.
func TestProcessSVGInput(t *testing.T) {
	src := []byte(svgSource)

	if !isSVG(src) {
		t.Fatal("isSVG did not recognize the SVG source")
	}

	results, err := Process(context.Background(), src, AllPresets())
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("preset %q failed on SVG input: %v", r.Preset.Name, r.Err)
			continue
		}
		if r.Preset.Kind == KindFaviconPack {
			assertFaviconPack(t, r)
			continue
		}
		if len(r.Data) == 0 {
			t.Errorf("preset %q produced empty output", r.Preset.Name)
			continue
		}

		if r.Preset.Resizes() {
			// Vector source rendered straight to the target box.
			if r.Width != r.Preset.Width || r.Height != r.Preset.Height {
				t.Errorf("preset %q reported %dx%d, want %dx%d", r.Preset.Name, r.Width, r.Height, r.Preset.Width, r.Preset.Height)
			}
			continue
		}

		// Density-bumped render: a 64px-wide SVG at 4× density should produce
		// noticeably more than its intrinsic size.
		if r.Width <= 64 {
			t.Errorf("preset %q rendered SVG at %dx%d — density bump did not apply", r.Preset.Name, r.Width, r.Height)
		}
	}
}

func TestProcessContextCancel(t *testing.T) {
	src := makeSourcePNG(t, 800, 600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := Process(ctx, src, AllPresets())
	if err == nil {
		t.Error("Process with a cancelled context returned nil error, want context.Canceled")
	}
}
