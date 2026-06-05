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
	if err := Startup(); err != nil {
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

func TestProcessContextCancel(t *testing.T) {
	src := makeSourcePNG(t, 800, 600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	_, err := Process(ctx, src, AllPresets())
	if err == nil {
		t.Error("Process with a cancelled context returned nil error, want context.Canceled")
	}
}
