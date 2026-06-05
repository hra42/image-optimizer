//go:build vips

package processor

import (
	"fmt"

	"github.com/davidbyttow/govips/v2/vips"
)

// processImage runs a single preset over the source buffer: load from memory
// (no disk), resize/crop to the target dimensions, strip metadata, and export
// with the preset's tuned encoding params. Any failure is returned inside the
// Result so the orchestrator can keep the other presets going.
func processImage(buf []byte, p Preset) Result {
	res := Result{Preset: p}

	var (
		img *vips.ImageRef
		err error
	)
	if p.Resizes() {
		// Crop-to-fill at exact dimensions. The thumbnail path shrinks with
		// Lanczos3 internally and guarantees an exact Width×Height via a centre
		// crop (SizeBoth lets it both up- and down-scale to hit the target box).
		img, err = vips.NewThumbnailWithSizeFromBuffer(buf, p.Width, p.Height, vips.InterestingCentre, vips.SizeBoth)
	} else {
		// website_* presets keep the source dimensions.
		img, err = vips.NewImageFromBuffer(buf)
	}
	if err != nil {
		res.Err = fmt.Errorf("load %q: %w", p.Name, err)
		return res
	}
	defer img.Close()

	// Bake EXIF orientation before stripping metadata, then strip it.
	_ = img.AutoRotate()
	if err := img.RemoveMetadata(); err != nil {
		res.Err = fmt.Errorf("strip metadata %q: %w", p.Name, err)
		return res
	}

	out, err := export(img, p)
	if err != nil {
		res.Err = fmt.Errorf("export %q: %w", p.Name, err)
		return res
	}

	res.Data = out
	res.Width = img.Width()
	res.Height = img.Height()
	return res
}

// export encodes the image into the preset's format with its tuned parameters.
// StripMetadata is also set on every params struct as a belt-and-suspenders
// alongside RemoveMetadata above.
func export(img *vips.ImageRef, p Preset) ([]byte, error) {
	switch p.Format {
	case FormatJPEG:
		out, _, err := img.ExportJpeg(&vips.JpegExportParams{
			Quality:       p.Quality,
			Interlace:     p.Progressive,
			StripMetadata: true,
		})
		return out, err
	case FormatWebP:
		out, _, err := img.ExportWebp(&vips.WebpExportParams{
			Quality:         p.Quality,
			ReductionEffort: p.Effort,
			StripMetadata:   true,
		})
		return out, err
	case FormatPNG:
		out, _, err := img.ExportPng(&vips.PngExportParams{
			Compression:   p.Compression,
			StripMetadata: true,
		})
		return out, err
	case FormatAVIF:
		out, _, err := img.ExportAvif(&vips.AvifExportParams{
			Quality:       p.Quality,
			Effort:        p.Effort,
			StripMetadata: true,
		})
		return out, err
	default:
		return nil, fmt.Errorf("unsupported format %q", p.Format)
	}
}
