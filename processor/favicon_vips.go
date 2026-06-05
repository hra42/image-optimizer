//go:build vips

package processor

import (
	"fmt"

	"github.com/davidbyttow/govips/v2/vips"
)

// faviconSizes are the square PNG sizes the pack renders. The ICO is assembled
// from the 16/32/48 subset; apple-touch and android-chrome use the larger ones.
var faviconSizes = []struct {
	px   int
	name string
}{
	{16, "favicon-16x16.png"},
	{32, "favicon-32x32.png"},
	{48, "favicon-48x48.png"},
	{180, "apple-touch-icon.png"},
	{192, "android-chrome-192x192.png"},
	{512, "android-chrome-512x512.png"},
}

// icoSizes are the dimensions embedded in favicon.ico.
var icoSizes = []int{16, 32, 48}

// buildFaviconPack renders the full favicon pack from the source buffer. Each
// size is produced by a centre-crop-to-square thumbnail (InterestingCentre +
// SizeBoth), matching the cropping behaviour of the fixed-size image presets,
// then PNG-encoded. The .ico is assembled from the 16/32/48 PNGs; the manifest
// and README are static. Any failure aborts the whole pack (returned as the
// preset's Result.Err) since a partial favicon set is not useful.
func buildFaviconPack(buf []byte, p Preset) Result {
	res := Result{Preset: p}

	// Render every required size once and cache the PNG bytes, so the ICO can
	// reuse the 16/32/48 renders without re-encoding.
	pngBySize := make(map[int][]byte)
	var files []OutputFile

	render := func(px int) ([]byte, error) {
		if existing, ok := pngBySize[px]; ok {
			return existing, nil
		}
		img, err := vips.NewThumbnailWithSizeFromBuffer(buf, px, px, vips.InterestingCentre, vips.SizeBoth)
		if err != nil {
			return nil, fmt.Errorf("render %dpx: %w", px, err)
		}
		defer img.Close()

		_ = img.AutoRotate()
		if err := img.RemoveMetadata(); err != nil {
			return nil, fmt.Errorf("strip metadata %dpx: %w", px, err)
		}

		out, _, err := img.ExportPng(&vips.PngExportParams{
			Compression:   p.Compression,
			StripMetadata: true,
		})
		if err != nil {
			return nil, fmt.Errorf("encode %dpx: %w", px, err)
		}
		pngBySize[px] = out
		return out, nil
	}

	// Named PNG members.
	for _, s := range faviconSizes {
		data, err := render(s.px)
		if err != nil {
			res.Err = err
			return res
		}
		files = append(files, OutputFile{Name: s.name, Data: data})
	}

	// favicon.ico from the small renders.
	icoImages := make([]icoImage, 0, len(icoSizes))
	for _, px := range icoSizes {
		data, err := render(px)
		if err != nil {
			res.Err = err
			return res
		}
		icoImages = append(icoImages, icoImage{size: px, png: data})
	}
	ico, err := buildICO(icoImages)
	if err != nil {
		res.Err = fmt.Errorf("build favicon.ico: %w", err)
		return res
	}
	files = append(files, OutputFile{Name: "favicon.ico", Data: ico})

	// Static text members.
	files = append(files, OutputFile{Name: "site.webmanifest", Data: faviconManifestBytes()})
	files = append(files, OutputFile{Name: "README.txt", Data: faviconReadmeBytes()})

	res.Files = files
	return res
}
