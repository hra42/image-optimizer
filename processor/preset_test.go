package processor

import "testing"

// These tests are tag-free so they run on every local `go test ./...` without
// libvips. They lock the registry to the HRA-163 spec table.

func TestAllPresetsMatchSpec(t *testing.T) {
	want := map[string]Preset{
		"website_webp":       {Name: "website_webp", Format: FormatWebP, Quality: 80},
		"website_avif":       {Name: "website_avif", Format: FormatAVIF, Quality: 60, Effort: 4},
		"instagram_square":   {Name: "instagram_square", Format: FormatJPEG, Width: 1080, Height: 1080, Quality: 80, Progressive: true},
		"instagram_portrait": {Name: "instagram_portrait", Format: FormatJPEG, Width: 1080, Height: 1350, Quality: 80, Progressive: true},
		"linkedin":           {Name: "linkedin", Format: FormatJPEG, Width: 1200, Height: 627, Quality: 80, Progressive: true},
		"twitter":            {Name: "twitter", Format: FormatJPEG, Width: 1200, Height: 675, Quality: 80, Progressive: true},
		"og_image":           {Name: "og_image", Format: FormatPNG, Width: 1200, Height: 630, Compression: 6},
	}

	got := AllPresets()
	if len(got) != len(want) {
		t.Fatalf("AllPresets() returned %d presets, want %d", len(got), len(want))
	}

	for _, p := range got {
		w, ok := want[p.Name]
		if !ok {
			t.Errorf("unexpected preset %q in registry", p.Name)
			continue
		}
		if p != w {
			t.Errorf("preset %q = %+v, want %+v", p.Name, p, w)
		}
		delete(want, p.Name)
	}
	for name := range want {
		t.Errorf("missing preset %q from registry", name)
	}
}

func TestAllPresetsReturnsCopy(t *testing.T) {
	a := AllPresets()
	if len(a) == 0 {
		t.Fatal("AllPresets() returned empty")
	}
	a[0].Name = "mutated"
	if b := AllPresets(); b[0].Name == "mutated" {
		t.Error("AllPresets() leaked the backing array; callers can mutate the registry")
	}
}

func TestPresetByName(t *testing.T) {
	if p, ok := PresetByName("instagram_square"); !ok {
		t.Error("PresetByName(instagram_square) miss, want hit")
	} else if p.Width != 1080 || p.Height != 1080 {
		t.Errorf("instagram_square dims = %dx%d, want 1080x1080", p.Width, p.Height)
	}

	if _, ok := PresetByName("does_not_exist"); ok {
		t.Error("PresetByName(does_not_exist) hit, want miss")
	}
}

func TestResizes(t *testing.T) {
	website, _ := PresetByName("website_webp")
	if website.Resizes() {
		t.Error("website_webp should keep original dimensions (Resizes()=false)")
	}
	ig, _ := PresetByName("instagram_square")
	if !ig.Resizes() {
		t.Error("instagram_square should crop to fixed dimensions (Resizes()=true)")
	}
}
