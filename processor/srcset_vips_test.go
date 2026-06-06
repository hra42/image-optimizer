//go:build vips

package processor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/davidbyttow/govips/v2/vips"
)

// assertSrcsetPack checks a srcset pack: every expected width is present in all
// three formats, each render decodes to the named width with the source aspect
// ratio preserved (NOT cropped to a square), and the index.html + README members
// exist and name the rendered files.
func assertSrcsetPack(t *testing.T, r Result, srcW, srcH int) {
	t.Helper()

	if len(r.Files) == 0 {
		t.Fatalf("srcset pack produced no files")
	}

	byName := make(map[string][]byte, len(r.Files))
	for _, f := range r.Files {
		if len(f.Data) == 0 {
			t.Errorf("srcset member %q is empty", f.Name)
		}
		byName[f.Name] = f.Data
	}

	wantWidths := srcsetTargetWidths(srcW)
	aspect := float64(srcW) / float64(srcH)

	for _, w := range wantWidths {
		for _, f := range []Format{FormatAVIF, FormatWebP, FormatJPEG} {
			name := srcsetMember(w, f)
			data, ok := byName[name]
			if !ok {
				t.Errorf("srcset pack missing %q", name)
				continue
			}
			img, err := vips.NewImageFromBuffer(data)
			if err != nil {
				t.Errorf("srcset member %q failed to decode: %v", name, err)
				continue
			}
			if img.Width() != w {
				t.Errorf("srcset member %q decoded width %d, want %d", name, img.Width(), w)
			}
			// Aspect ratio preserved (not cropped). Allow ±1px of rounding on the
			// derived height.
			wantH := int(float64(w)/aspect + 0.5)
			if d := img.Height() - wantH; d < -1 || d > 1 {
				t.Errorf("srcset member %q is %dx%d, want ~%dx%d (aspect not preserved / cropped)",
					name, img.Width(), img.Height(), w, wantH)
			}
			img.Close()
		}
	}

	// index.html must exist and reference each rendered width.
	html, ok := byName["index.html"]
	if !ok {
		t.Error("srcset pack missing index.html")
	} else {
		s := string(html)
		for _, w := range wantWidths {
			if !strings.Contains(s, fmt.Sprintf("%dw", w)) {
				t.Errorf("index.html does not reference width %dw", w)
			}
		}
	}
	if _, ok := byName["README.txt"]; !ok {
		t.Error("srcset pack missing README.txt")
	}
}

// assertSrcsetPackStructure checks a pack has at least one rendered width in all
// three formats plus the snippet and README, without asserting exact dimensions.
// Used for sources (e.g. SVG) whose rendered width is density-derived.
func assertSrcsetPackStructure(t *testing.T, r Result) {
	t.Helper()

	if len(r.Files) == 0 {
		t.Fatalf("srcset pack produced no files")
	}
	names := make(map[string]bool, len(r.Files))
	var haveAVIF, haveWebP, haveJPEG bool
	for _, f := range r.Files {
		if len(f.Data) == 0 {
			t.Errorf("srcset member %q is empty", f.Name)
		}
		names[f.Name] = true
		switch {
		case strings.HasSuffix(f.Name, ".avif"):
			haveAVIF = true
		case strings.HasSuffix(f.Name, ".webp"):
			haveWebP = true
		case strings.HasSuffix(f.Name, ".jpg"):
			haveJPEG = true
		}
	}
	if !haveAVIF || !haveWebP || !haveJPEG {
		t.Errorf("srcset pack missing a format: avif=%v webp=%v jpeg=%v", haveAVIF, haveWebP, haveJPEG)
	}
	for _, n := range []string{"index.html", "README.txt"} {
		if !names[n] {
			t.Errorf("srcset pack missing %q", n)
		}
	}
}

// TestSrcsetTargetWidths covers the no-upscale filter and the small-source
// fallback purely with arithmetic (no libvips needed, but kept here next to the
// pack tests).
func TestSrcsetTargetWidths(t *testing.T) {
	cases := []struct {
		srcW int
		want []int
	}{
		{srcW: 3000, want: []int{480, 768, 1024, 1366, 1920, 3000}}, // all + source
		{srcW: 1920, want: []int{480, 768, 1024, 1366, 1920}},       // 1920 not < 1920; source caps it
		{srcW: 1500, want: []int{480, 768, 1024, 1366, 1500}},       // drops 1920, adds source
		{srcW: 500, want: []int{480, 500}},                          // only 480 fits, plus source
		{srcW: 300, want: []int{300}},                               // nothing fits → source only
	}
	for _, c := range cases {
		got := srcsetTargetWidths(c.srcW)
		if len(got) != len(c.want) {
			t.Errorf("srcsetTargetWidths(%d) = %v, want %v", c.srcW, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("srcsetTargetWidths(%d) = %v, want %v", c.srcW, got, c.want)
				break
			}
		}
	}
}

// TestBuildSrcsetPack runs the full assembler over a landscape source and checks
// the pack via assertSrcsetPack.
func TestBuildSrcsetPack(t *testing.T) {
	const srcW, srcH = 2000, 1200
	src := makeSourcePNG(t, srcW, srcH)
	p, _ := PresetByName("srcset_web")

	r := buildSrcsetPack(src, p)
	if r.Err != nil {
		t.Fatalf("buildSrcsetPack: %v", r.Err)
	}
	if len(r.Data) != 0 {
		t.Error("srcset pack should leave Result.Data nil (it fills Files)")
	}
	assertSrcsetPack(t, r, srcW, srcH)
}

// TestBuildSrcsetPackSmallSource verifies a source smaller than every preset
// width still yields a single-width pack at the source width.
func TestBuildSrcsetPackSmallSource(t *testing.T) {
	const srcW, srcH = 300, 200
	src := makeSourcePNG(t, srcW, srcH)
	p, _ := PresetByName("srcset_web")

	r := buildSrcsetPack(src, p)
	if r.Err != nil {
		t.Fatalf("buildSrcsetPack: %v", r.Err)
	}
	assertSrcsetPack(t, r, srcW, srcH)

	// Exactly one width (the source width) × 3 formats + index.html + README.
	if len(r.Files) != 3+2 {
		var names []string
		for _, f := range r.Files {
			names = append(names, f.Name)
		}
		t.Errorf("small-source pack has %d files (%v), want 5", len(r.Files), names)
	}
}
