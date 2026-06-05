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
