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
	// Pack presets produce a set of files rather than a single image.
	if p.Kind == KindFaviconPack {
		return buildFaviconPack(buf, p)
	}
	// Bundle presets (document PDFs) consume all files at once and must go
	// through ProcessBundle, not the per-file path. The handler partitions them
	// out before they reach here; this guards against a caller that doesn't.
	if p.IsBundle() {
		return Result{Preset: p, Err: fmt.Errorf("preset %q is a bundle preset; use ProcessBundle", p.Name)}
	}

	res := Result{Preset: p}

	var (
		img *vips.ImageRef
		err error
	)
	switch {
	case p.Resizes():
		// Crop-to-fill at exact dimensions. The thumbnail path shrinks with
		// Lanczos3 internally and guarantees an exact Width×Height via a centre
		// crop (SizeBoth lets it both up- and down-scale to hit the target box).
		// For SVG input this renders the vector source straight to the target
		// raster size, so the result stays crisp.
		img, err = vips.NewThumbnailWithSizeFromBuffer(buf, p.Width, p.Height, vips.InterestingCentre, vips.SizeBoth)
	case isSVG(buf):
		// "Keep original size" presets have no target dimensions, but SVG is
		// vector — libvips would otherwise rasterize at the document's intrinsic
		// size, which for icon SVGs is often tiny. Render at a higher density so
		// the output is a usable size while preserving the author's proportions.
		img, err = loadSVGAtDensity(buf, svgRasterDensity)
	default:
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

	// Compress presets carry FormatAuto + a Tier: resolve the output format from
	// the input and swap in the tier-tuned codec knobs. The resolved preset is
	// stored on the Result so the download handler emits the correct extension
	// (it reads r.Preset.Format).
	if p.Format == FormatAuto {
		p = compressParams(p, resolveFormat(buf))
		res.Preset = p
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

// resolveFormat maps a source buffer to the concrete output format a compress
// preset should keep. JPEG/PNG/WebP/AVIF round-trip to themselves ("no surprise
// file type"); inputs with no matching output codec degrade sensibly: SVG → PNG
// (vector to lossless raster, preserving transparency), everything else (HEIC,
// GIF, …) → JPEG (the safe photographic default).
func resolveFormat(buf []byte) Format {
	switch vips.DetermineImageType(buf) {
	case vips.ImageTypeJPEG:
		return FormatJPEG
	case vips.ImageTypePNG:
		return FormatPNG
	case vips.ImageTypeWEBP:
		return FormatWebP
	case vips.ImageTypeAVIF:
		return FormatAVIF
	case vips.ImageTypeSVG:
		return FormatPNG
	default:
		return FormatJPEG
	}
}

// compressParams returns a copy of p with its Format set to the resolved format
// and the tier-tuned codec knobs filled in for that format. The tiers are kept
// honest — Max savings is visibly smaller than Best quality for the lossy
// formats. PNG has no quality knob, so Best/Balanced are identical (max lossless
// compression) and only Max diverges, via a lossy palette quantization.
func compressParams(p Preset, f Format) Preset {
	p.Format = f
	switch f {
	case FormatJPEG:
		p.Progressive = true
		p.Quality = map[CompressTier]int{TierBest: 90, TierBalanced: 80, TierMax: 65}[p.Tier]
	case FormatWebP:
		p.Quality = map[CompressTier]int{TierBest: 88, TierBalanced: 78, TierMax: 62}[p.Tier]
		p.Effort = map[CompressTier]int{TierBest: 4, TierBalanced: 4, TierMax: 5}[p.Tier]
	case FormatAVIF:
		p.Quality = map[CompressTier]int{TierBest: 70, TierBalanced: 55, TierMax: 40}[p.Tier]
		p.Effort = map[CompressTier]int{TierBest: 4, TierBalanced: 4, TierMax: 5}[p.Tier]
	case FormatPNG:
		// Always maximum lossless compression; only Max savings adds a lossy
		// palette (handled in export when p.Tier == TierMax).
		p.Compression = 9
	}
	return p
}

// svgRasterDensity is the DPI used to rasterize SVG input for "keep original
// size" presets. librsvg's default is 72 DPI; 288 is a 4× bump, which turns a
// nominally 256px-wide SVG into ~1024px of crisp raster while preserving the
// SVG author's intended proportions.
const svgRasterDensity = 288

// isSVG reports whether buf is an SVG document, reusing govips' detection so we
// match exactly what the loader would dispatch to vips_svgload.
func isSVG(buf []byte) bool {
	return vips.DetermineImageType(buf) == vips.ImageTypeSVG
}

// loadSVGAtDensity rasterizes an SVG buffer at the given DPI. Non-SVG callers
// should not reach this — it sets the svgload-specific dpi import option.
func loadSVGAtDensity(buf []byte, density int) (*vips.ImageRef, error) {
	params := vips.NewImportParams()
	params.Density.Set(density)
	return vips.LoadImageFromBuffer(buf, params)
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
		lossless, _, err := img.ExportPng(&vips.PngExportParams{
			Compression:   p.Compression,
			StripMetadata: true,
		})
		if err != nil {
			return nil, err
		}
		// Max-savings compress on PNG: also try a palette (lossy) encode, since
		// PNG has no quality knob. But palette + dithering can *inflate* smooth
		// images (a gradient quantized to 256 colours compresses worse than the
		// original), so keep whichever encode is actually smaller — "max savings"
		// must never be larger than the lossless tiers. Other PNG presets keep the
		// plain lossless path.
		if p.Tier == TierMax {
			palette, _, perr := img.ExportPng(&vips.PngExportParams{
				Compression:   p.Compression,
				StripMetadata: true,
				Palette:       true,
				Quality:       90,
			})
			if perr == nil && len(palette) < len(lossless) {
				return palette, nil
			}
		}
		return lossless, err
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
