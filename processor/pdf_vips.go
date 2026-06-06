//go:build vips

package processor

import (
	"fmt"

	"github.com/davidbyttow/govips/v2/vips"
)

// renderDocumentPage attention-crops one source buffer to the preset's per-page
// box (the same crop path as the fixed-size image presets) and encodes it via the
// shared export(). The result is one ready-to-place PDF page. Mirrors the render
// step in favicon_vips.go.
func renderDocumentPage(buf []byte, p Preset) (pdfPage, error) {
	img, err := vips.NewThumbnailWithSizeFromBuffer(buf, p.Width, p.Height, cropInteresting, vips.SizeBoth)
	if err != nil {
		return pdfPage{}, fmt.Errorf("render page %q: %w", p.Name, err)
	}
	defer img.Close()

	_ = img.AutoRotate()
	if err := img.RemoveMetadata(); err != nil {
		return pdfPage{}, fmt.Errorf("strip metadata %q: %w", p.Name, err)
	}

	out, err := export(img, p)
	if err != nil {
		return pdfPage{}, fmt.Errorf("encode page %q: %w", p.Name, err)
	}

	return pdfPage{data: out, format: p.Format, wPx: img.Width(), hPx: img.Height()}, nil
}
