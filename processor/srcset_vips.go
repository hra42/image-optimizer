//go:build vips

package processor

import (
	"fmt"

	"github.com/davidbyttow/govips/v2/vips"
)

// srcsetWidths are the target widths (in px) the responsive pack renders, sorted
// ascending. Device-breakpoint aligned: small phones through desktop/retina. A
// width >= the source width is skipped (no upscaling — we never invent pixels);
// if every width is skipped the source width itself is used so a small source
// still yields a usable pack.
var srcsetWidths = []int{480, 768, 1024, 1366, 1920}

// srcsetFormats are the per-width encodings, modern-first. JPEG is the universal
// <img> fallback; AVIF and WebP are the smaller modern <source>s.
var srcsetFormats = []Format{FormatAVIF, FormatWebP, FormatJPEG}

// srcsetMaxHeight is an effectively-unbounded height passed to the thumbnail op
// so width drives the resize and aspect ratio is preserved (the libvips idiom
// for "fit to this width": a huge height box + SizeDown). It is far larger than
// any realistic image height.
const srcsetMaxHeight = 1 << 20

// buildSrcsetPack renders the responsive image pack from the source buffer: each
// target width (that doesn't upscale the source) is rendered once preserving the
// source aspect ratio — NOT cropped — then encoded in AVIF, WebP, and JPEG. A
// paste-in <picture> snippet (index.html) and a README round out the folder. Any
// hard failure aborts the whole pack (returned as the preset's Result.Err), since
// a partial set with a snippet referencing missing files is worse than nothing.
func buildSrcsetPack(buf []byte, p Preset) Result {
	res := Result{Preset: p}

	srcW, err := srcsetSourceWidth(buf)
	if err != nil {
		res.Err = fmt.Errorf("read source width: %w", err)
		return res
	}

	widths := srcsetTargetWidths(srcW)

	var files []OutputFile
	for _, w := range widths {
		for _, f := range srcsetFormats {
			// Render fresh from buf per (width, format): a govips ImageRef loaded
			// from a buffer wraps a sequential reader that can only be consumed
			// once, so re-exporting one ImageRef to several formats fails with an
			// "out of order read". Re-thumbnailing from buf is the favicon pack's
			// proven pattern and stays cheap thanks to libvips shrink-on-load.
			out, err := renderSrcsetVariant(buf, w, srcsetFormatPreset(p, f))
			if err != nil {
				res.Err = fmt.Errorf("render %dw %s: %w", w, f, err)
				return res
			}
			files = append(files, OutputFile{Name: srcsetMember(w, f), Data: out})
		}
	}

	// Static members: the <picture> snippet (referencing exactly the widths
	// rendered) and the README.
	files = append(files, OutputFile{Name: "index.html", Data: srcsetSnippetBytes(widths)})
	files = append(files, OutputFile{Name: "README.txt", Data: srcsetReadmeBytes()})

	res.Files = files
	return res
}

// renderSrcsetVariant resizes buf to width w preserving aspect ratio (no crop,
// no upscale) and encodes it with preset p. The width filter in buildSrcsetPack
// guarantees w <= source width, so SizeDown lands exactly at w. Metadata is baked
// (orientation) then stripped, matching the other render paths.
func renderSrcsetVariant(buf []byte, w int, p Preset) ([]byte, error) {
	img, err := vips.NewThumbnailWithSizeFromBuffer(buf, w, srcsetMaxHeight, vips.InterestingNone, vips.SizeDown)
	if err != nil {
		return nil, err
	}
	defer img.Close()

	_ = img.AutoRotate()
	if err := img.RemoveMetadata(); err != nil {
		return nil, fmt.Errorf("strip metadata: %w", err)
	}
	return export(img, p)
}

// srcsetFormatPreset returns a copy of the pack's master preset p with its Format
// set to f. Every format shares the pack's master Quality/Effort/Progressive
// knobs, which export() reads directly.
func srcsetFormatPreset(p Preset, f Format) Preset {
	p.Format = f
	return p
}

// srcsetSourceWidth returns the pixel width of the source buffer without keeping
// the image around. SVG is rasterized at the same density the rest of the
// pipeline uses so its intrinsic-vs-rendered width is consistent.
func srcsetSourceWidth(buf []byte) (int, error) {
	var (
		img *vips.ImageRef
		err error
	)
	if isSVG(buf) {
		img, err = loadSVGAtDensity(buf, svgRasterDensity)
	} else {
		img, err = vips.NewImageFromBuffer(buf)
	}
	if err != nil {
		return 0, err
	}
	w := img.Width()
	img.Close()
	return w, nil
}

// srcsetTargetWidths returns the widths to render for a source of width srcW:
// every preset width strictly smaller than the source (no upscaling), sorted
// ascending. If the source is smaller than the smallest preset width, the source
// width itself is used so a small source still yields a (single-width) pack.
func srcsetTargetWidths(srcW int) []int {
	var widths []int
	for _, w := range srcsetWidths {
		if w < srcW {
			widths = append(widths, w)
		}
	}
	if len(widths) == 0 {
		return []int{srcW}
	}
	// Every kept width is strictly < srcW, so add the source width itself as the
	// top of the set: the pack always offers a full-resolution option (matching
	// what a user gets from any "keep original" preset) and the <picture>
	// fallback src points at the real source resolution.
	return append(widths, srcW)
}
