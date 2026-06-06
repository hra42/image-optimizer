package processor

// Tag-free tests for the srcset pack's static asset generators. These run in a
// plain `go test ./...` without libvips.

import (
	"strings"
	"testing"
)

func TestSrcsetExt(t *testing.T) {
	cases := map[Format]string{
		FormatJPEG: "jpg",
		FormatWebP: "webp",
		FormatAVIF: "avif",
		FormatPNG:  "png",
		Format("nonsense"): "bin",
	}
	for f, want := range cases {
		if got := srcsetExt(f); got != want {
			t.Errorf("srcsetExt(%q) = %q, want %q", f, got, want)
		}
	}
}

func TestSrcsetMember(t *testing.T) {
	if got := srcsetMember(768, FormatWebP); got != "image-768w.webp" {
		t.Errorf("srcsetMember(768, webp) = %q, want image-768w.webp", got)
	}
	if got := srcsetMember(1920, FormatJPEG); got != "image-1920w.jpg" {
		t.Errorf("srcsetMember(1920, jpeg) = %q, want image-1920w.jpg", got)
	}
}

func TestSrcsetSnippetNamesRenderedFiles(t *testing.T) {
	widths := []int{480, 768, 1024}
	snippet := string(srcsetSnippetBytes(widths))

	// One <source> per modern format, in modern-first order, plus the JPEG <img>.
	if !strings.Contains(snippet, `<source type="image/avif"`) {
		t.Error("snippet missing AVIF <source>")
	}
	if !strings.Contains(snippet, `<source type="image/webp"`) {
		t.Error("snippet missing WebP <source>")
	}
	if strings.Index(snippet, "image/avif") > strings.Index(snippet, "image/webp") {
		t.Error("AVIF <source> should come before WebP <source> (modern-first)")
	}

	// Every rendered width must be referenced in each format, with its NNNw descriptor.
	for _, w := range widths {
		for _, f := range []Format{FormatAVIF, FormatWebP, FormatJPEG} {
			member := srcsetMember(w, f)
			if !strings.Contains(snippet, member) {
				t.Errorf("snippet does not reference %q", member)
			}
		}
	}

	// The <img> fallback src must be the largest width's JPEG.
	largestJPEG := srcsetMember(1024, FormatJPEG)
	if !strings.Contains(snippet, `<img src="`+largestJPEG+`"`) {
		t.Errorf("snippet <img> src is not %q", largestJPEG)
	}

	// A width that was NOT rendered must never appear.
	if strings.Contains(snippet, "1920w") {
		t.Error("snippet references a width that was not rendered (1920w)")
	}
}

func TestSrcsetSnippetEmptyWidths(t *testing.T) {
	if got := string(srcsetSnippetBytes(nil)); !strings.Contains(got, "no widths") {
		t.Errorf("empty-widths snippet = %q, want a no-widths comment", got)
	}
}

func TestSrcsetReadmeNonEmpty(t *testing.T) {
	if len(srcsetReadmeBytes()) == 0 {
		t.Error("srcsetReadmeBytes is empty")
	}
}

func TestSrcsetWebPresetRegistered(t *testing.T) {
	p, ok := PresetByName("srcset_web")
	if !ok {
		t.Fatal("srcset_web preset not found in registry")
	}
	if p.Kind != KindSrcsetPack {
		t.Errorf("srcset_web Kind = %v, want KindSrcsetPack", p.Kind)
	}
	if p.IsBundle() {
		t.Error("srcset_web should not be a bundle preset")
	}
}
