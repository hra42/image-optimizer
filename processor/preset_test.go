package processor

import "testing"

// These tests are tag-free so they run on every local `go test ./...` without
// libvips. They lock the registry to the HRA-163 spec table.

func TestAllPresetsMatchSpec(t *testing.T) {
	want := map[string]Preset{
		"website_webp":            {Name: "website_webp", Format: FormatWebP, Quality: 80},
		"website_avif":            {Name: "website_avif", Format: FormatAVIF, Quality: 60, Effort: 4},
		"jpeg_original":           {Name: "jpeg_original", Format: FormatJPEG, Quality: 80, Progressive: true},
		"png_original":            {Name: "png_original", Format: FormatPNG, Compression: 6},
		"convert_jpeg":            {Name: "convert_jpeg", Format: FormatJPEG, Quality: 92, Progressive: true},
		"convert_png":             {Name: "convert_png", Format: FormatPNG, Compression: 6},
		"convert_webp":            {Name: "convert_webp", Format: FormatWebP, Quality: 90},
		"convert_avif":            {Name: "convert_avif", Format: FormatAVIF, Quality: 80, Effort: 4},
		"compress_best":           {Name: "compress_best", Format: FormatAuto, Tier: TierBest},
		"compress_balanced":       {Name: "compress_balanced", Format: FormatAuto, Tier: TierBalanced},
		"compress_max":            {Name: "compress_max", Format: FormatAuto, Tier: TierMax},
		"instagram_square":        {Name: "instagram_square", Format: FormatJPEG, Width: 1080, Height: 1080, Quality: 80, Progressive: true},
		"instagram_portrait":      {Name: "instagram_portrait", Format: FormatJPEG, Width: 1080, Height: 1350, Quality: 80, Progressive: true},
		"instagram_story":         {Name: "instagram_story", Format: FormatJPEG, Width: 1080, Height: 1920, Quality: 80, Progressive: true},
		"linkedin":                {Name: "linkedin", Format: FormatJPEG, Width: 1200, Height: 627, Quality: 80, Progressive: true},
		"linkedin_profile_banner": {Name: "linkedin_profile_banner", Format: FormatJPEG, Width: 1584, Height: 396, Quality: 80, Progressive: true},
		"linkedin_company_banner": {Name: "linkedin_company_banner", Format: FormatJPEG, Width: 1128, Height: 191, Quality: 80, Progressive: true},
		"twitter":                 {Name: "twitter", Format: FormatJPEG, Width: 1200, Height: 675, Quality: 80, Progressive: true},
		"facebook_post":           {Name: "facebook_post", Format: FormatJPEG, Width: 1200, Height: 630, Quality: 80, Progressive: true},
		"pinterest_pin":           {Name: "pinterest_pin", Format: FormatJPEG, Width: 1000, Height: 1500, Quality: 80, Progressive: true},
		"og_image":                {Name: "og_image", Format: FormatPNG, Width: 1200, Height: 630, Compression: 6},
		"favicon":                 {Name: "favicon", Kind: KindFaviconPack, Format: FormatPNG, Compression: 6},
		"thumbnail":               {Name: "thumbnail", Format: FormatPNG, Width: 400, Height: 400, Compression: 6},
		"email_header":            {Name: "email_header", Format: FormatJPEG, Width: 600, Height: 200, Quality: 80, Progressive: true},
		"web_banner":              {Name: "web_banner", Format: FormatJPEG, Width: 1920, Height: 480, Quality: 80, Progressive: true},
		"linkedin_doc_portrait":   {Name: "linkedin_doc_portrait", Kind: KindDocumentPDF, Format: FormatJPEG, Width: 1080, Height: 1350, Quality: 85, Progressive: true},
		"linkedin_doc_square":     {Name: "linkedin_doc_square", Kind: KindDocumentPDF, Format: FormatJPEG, Width: 1080, Height: 1080, Quality: 85, Progressive: true},
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

func TestIsBundle(t *testing.T) {
	doc, _ := PresetByName("linkedin_doc_portrait")
	if !doc.IsBundle() {
		t.Error("linkedin_doc_portrait should be a bundle preset (IsBundle()=true)")
	}
	ig, _ := PresetByName("instagram_square")
	if ig.IsBundle() {
		t.Error("instagram_square should not be a bundle preset (IsBundle()=false)")
	}
	fav, _ := PresetByName("favicon")
	if fav.IsBundle() {
		t.Error("favicon (pack, not bundle) should have IsBundle()=false")
	}
}
