package processor

// Tag-free: exercises buildDocumentPDF without libvips, by encoding tiny JPEGs
// with the stdlib. Mirrors how ico_test.go covers the tag-free ICO assembler.

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"regexp"
	"testing"
)

// makeJPEG builds a deterministic w×h solid-color JPEG in memory.
func makeJPEG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func TestBuildDocumentPDF(t *testing.T) {
	pages := []pdfPage{
		{data: makeJPEG(t, 100, 125, color.RGBA{R: 200, A: 255}), format: FormatJPEG, wPx: 100, hPx: 125},
		{data: makeJPEG(t, 100, 125, color.RGBA{G: 200, A: 255}), format: FormatJPEG, wPx: 100, hPx: 125},
		{data: makeJPEG(t, 100, 125, color.RGBA{B: 200, A: 255}), format: FormatJPEG, wPx: 100, hPx: 125},
	}

	out, err := buildDocumentPDF(pages)
	if err != nil {
		t.Fatalf("buildDocumentPDF: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("buildDocumentPDF returned empty output")
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- magic; got %q", out[:min(8, len(out))])
	}
	if !bytes.Contains(out, []byte("%%EOF")) {
		t.Error("output missing the PDF EOF trailer")
	}

	// One /Type /Page object per page (the catalog uses /Type /Pages, which the
	// \b after Page excludes).
	pageRe := regexp.MustCompile(`/Type\s*/Page\b`)
	if n := len(pageRe.FindAll(out, -1)); n != len(pages) {
		t.Errorf("found %d page objects, want %d", n, len(pages))
	}
}

func TestBuildDocumentPDFEmpty(t *testing.T) {
	if _, err := buildDocumentPDF(nil); err == nil {
		t.Error("buildDocumentPDF(nil) returned nil error, want a no-pages error")
	}
}

func TestBuildDocumentPDFUnsupportedFormat(t *testing.T) {
	pages := []pdfPage{
		{data: []byte("not really avif"), format: FormatAVIF, wPx: 10, hPx: 10},
	}
	if _, err := buildDocumentPDF(pages); err == nil {
		t.Error("buildDocumentPDF with an AVIF page returned nil error, want an unsupported-format error")
	}
}
