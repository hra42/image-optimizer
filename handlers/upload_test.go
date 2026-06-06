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
