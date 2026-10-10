package handlers

import (
	"testing"

	"github.com/hra42/image-optimizer/processor"
)

func TestPartitionPresets(t *testing.T) {
	mustPreset := func(name string) processor.Preset {
		p, ok := processor.PresetByName(name)
		if !ok {
			t.Fatalf("preset %q not found", name)
		}
		return p
	}

	in := []processor.Preset{
		mustPreset("instagram_square"),      // per-image
		mustPreset("linkedin_doc_portrait"), // bundle
		mustPreset("convert_jpeg"),          // per-image
		mustPreset("linkedin_doc_square"),   // bundle
	}

	image, bundle := partitionPresets(in)

	if len(image) != 2 || image[0].Name != "instagram_square" || image[1].Name != "convert_jpeg" {
		t.Errorf("image presets = %v, want [instagram_square convert_jpeg] in order", names(image))
	}
	if len(bundle) != 2 || bundle[0].Name != "linkedin_doc_portrait" || bundle[1].Name != "linkedin_doc_square" {
		t.Errorf("bundle presets = %v, want [linkedin_doc_portrait linkedin_doc_square] in order", names(bundle))
	}
}

func names(ps []processor.Preset) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

func TestParseFocals(t *testing.T) {
	t.Run("absent yields all-unset", func(t *testing.T) {
		got, err := parseFocals(nil, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("len = %d, want 3", len(got))
		}
		for i, f := range got {
			if f.Set {
				t.Errorf("focal[%d] should be unset", i)
			}
		}
	})

	t.Run("empty string yields all-unset", func(t *testing.T) {
		got, err := parseFocals([]string{"  "}, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i, f := range got {
			if f.Set {
				t.Errorf("focal[%d] should be unset", i)
			}
		}
	})

	t.Run("null entries stay unset, objects parse", func(t *testing.T) {
		got, err := parseFocals([]string{`[{"x":0.25,"y":0.75}, null, {"x":1,"y":0}]`}, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got[0].Set || got[0].X != 0.25 || got[0].Y != 0.75 {
			t.Errorf("focal[0] = %+v, want {0.25 0.75 true}", got[0])
		}
		if got[1].Set {
			t.Errorf("focal[1] should be unset (null), got %+v", got[1])
		}
		if !got[2].Set || got[2].X != 1 || got[2].Y != 0 {
			t.Errorf("focal[2] = %+v, want {1 0 true}", got[2])
		}
	})

	t.Run("out-of-range coords are clamped", func(t *testing.T) {
		got, err := parseFocals([]string{`[{"x":-0.5,"y":1.9}]`}, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got[0].X != 0 || got[0].Y != 1 {
			t.Errorf("focal[0] = %+v, want clamped to {0 1 true}", got[0])
		}
	})

	t.Run("length mismatch is rejected", func(t *testing.T) {
		if _, err := parseFocals([]string{`[{"x":0.5,"y":0.5}]`}, 2); err == nil {
			t.Error("expected error for length mismatch, got nil")
		}
	})

	t.Run("invalid JSON is rejected", func(t *testing.T) {
		if _, err := parseFocals([]string{`not json`}, 1); err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})
}

func TestParseAILabel(t *testing.T) {
	t.Run("absent disables label", func(t *testing.T) {
		got, err := parseAILabel(nil)
		if err != nil || got.Enabled() {
			t.Fatalf("got %+v, err %v; want disabled", got, err)
		}
	})

	t.Run("parses all options", func(t *testing.T) {
		got, err := parseAILabel(map[string][]string{
			"aiLabel":         {"modified"},
			"aiLabelColor":    {"white"},
			"aiLabelPosition": {"top-left"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Style != "modified" || got.Color != "white" || got.Position != "top-left" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("defaults optional choices", func(t *testing.T) {
		got, err := parseAILabel(map[string][]string{"aiLabel": {"generated"}})
		if err != nil {
			t.Fatal(err)
		}
		if got.Color != "black" || got.Position != "bottom-right" {
			t.Fatalf("got %+v", got)
		}
	})

	for _, values := range []map[string][]string{
		{"aiLabel": {"other"}},
		{"aiLabel": {"ai"}, "aiLabelColor": {"red"}},
		{"aiLabel": {"ai"}, "aiLabelPosition": {"center"}},
	} {
		if _, err := parseAILabel(values); err == nil {
			t.Errorf("expected error for %v", values)
		}
	}
}

func TestParseMattes(t *testing.T) {
	t.Run("absent yields all-unset", func(t *testing.T) {
		got, err := parseMattes(nil, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0].Set || got[1].Set {
			t.Fatalf("got %+v, want 2 unset mattes", got)
		}
	})

	t.Run("null entries stay unset, colors parse", func(t *testing.T) {
		got, err := parseMattes([]string{`["#ff8000", null]`}, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := (processor.Matte{R: 0xff, G: 0x80, B: 0x00, Set: true}); got[0] != want {
			t.Errorf("matte[0] = %+v, want %+v", got[0], want)
		}
		if got[1].Set {
			t.Errorf("matte[1] should be unset")
		}
	})

	for name, in := range map[string]string{
		"not an array":    `"#ffffff"`,
		"length mismatch": `["#ffffff"]`,
		"no hash":         `["ffffff", null]`,
		"short form":      `["#fff", null]`,
		"bad hex":         `["#gggggg", null]`,
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			if _, err := parseMattes([]string{in}, 2); err == nil {
				t.Errorf("parseMattes(%s) = nil error, want error", in)
			}
		})
	}
}
