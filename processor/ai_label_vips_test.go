//go:build vips

package processor

import (
	"bytes"
	"image/png"
	"testing"
)

func TestProcessImageAppliesAILabelInSelectedCorner(t *testing.T) {
	src := makeSourcePNG(t, 400, 300)
	basePreset := Preset{Name: "test", Format: FormatPNG, Compression: 6}

	plain := processImage(src, basePreset)
	if plain.Err != nil {
		t.Fatal(plain.Err)
	}
	stampedPreset := basePreset
	stampedPreset.AILabel = AILabel{Style: "generated", Color: "black", Position: "top-left"}
	stamped := processImage(src, stampedPreset)
	if stamped.Err != nil {
		t.Fatal(stamped.Err)
	}

	plainImage, err := png.Decode(bytes.NewReader(plain.Data))
	if err != nil {
		t.Fatal(err)
	}
	stampedImage, err := png.Decode(bytes.NewReader(stamped.Data))
	if err != nil {
		t.Fatal(err)
	}
	x, y, w, h := aiLabelPlacement(400, 300, 7459, 2363, "top-left")
	different := 0
	for py := 0; py < 300; py++ {
		for px := 0; px < 400; px++ {
			pr, pg, pb, pa := plainImage.At(px, py).RGBA()
			sr, sg, sb, sa := stampedImage.At(px, py).RGBA()
			if pr == sr && pg == sg && pb == sb && pa == sa {
				continue
			}
			different++
			if px < x || px >= x+w || py < y || py >= y+h {
				t.Fatalf("pixel changed outside stamp bounds at (%d,%d); bounds are (%d,%d) %dx%d", px, py, x, y, w, h)
			}
		}
	}
	if different < 100 {
		t.Fatalf("only %d pixels changed inside stamp bounds", different)
	}
}

func TestAILabelExportsSupportedFormats(t *testing.T) {
	src := makeSourcePNG(t, 400, 300)
	for _, format := range []Format{FormatJPEG, FormatPNG, FormatWebP, FormatAVIF} {
		t.Run(string(format), func(t *testing.T) {
			result := processImage(src, Preset{
				Name:        "test",
				Format:      format,
				Quality:     80,
				Compression: 6,
				Effort:      4,
				AILabel: AILabel{
					Style: "generated", Color: "white", Position: "bottom-right",
				},
			})
			if result.Err != nil {
				t.Fatal(result.Err)
			}
			if len(result.Data) == 0 {
				t.Fatal("empty output")
			}
		})
	}
}
