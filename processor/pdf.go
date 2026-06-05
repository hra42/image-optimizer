package processor

// This file is intentionally tag-free (no govips import), the document-PDF
// analog of ico.go: it assembles a multi-page PDF from already-encoded image
// bytes. The vips-side rendering that produces those bytes lives in the
// //go:build vips file pdf_vips.go. Keeping assembly here means it compiles and
// unit-tests locally without libvips.

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/go-pdf/fpdf"
)

// pdfPage is one already-encoded image destined for a single full-bleed PDF
// page. format must be a codec fpdf can embed (JPEG or PNG); wPx/hPx are the
// image's pixel dimensions, used both as the page size and the draw size so the
// mapping is 1px → 1pt (no letterboxing).
type pdfPage struct {
	data   []byte
	format Format
	wPx    int
	hPx    int
}

// pdfImageType maps our Format to the fpdf image-type string. fpdf can only
// embed JPEG and PNG (and GIF), so other formats are rejected — document
// presets are pinned to JPEG in the registry, this is the defensive guard.
func pdfImageType(f Format) (string, bool) {
	switch f {
	case FormatJPEG:
		return "JPEG", true
	case FormatPNG:
		return "PNG", true
	default:
		return "", false
	}
}

// buildDocumentPDF assembles a multi-page PDF, one page per pdfPage in slice
// order (= upload order). Each page is sized to its image and the image is drawn
// full-bleed, so there is no letterboxing. Returns an error on empty input or an
// unsupported page format.
func buildDocumentPDF(pages []pdfPage) ([]byte, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("document PDF: no pages")
	}

	// Unit "pt" gives a 1px → 1pt mapping. The initial size string is irrelevant
	// because every page overrides it via AddPageFormat.
	pdf := fpdf.New("P", "pt", "A4", "")
	// Without this a full-page-height image trips the auto page break and spills
	// onto a blank second page.
	pdf.SetAutoPageBreak(false, 0)

	for i, p := range pages {
		imgType, ok := pdfImageType(p.format)
		if !ok {
			return nil, fmt.Errorf("document PDF: page %d has unsupported format %q", i, p.format)
		}
		w, h := float64(p.wPx), float64(p.hPx)

		pdf.AddPageFormat("P", fpdf.SizeType{Wd: w, Ht: h})

		// Unique name per page: fpdf returns the already-registered image (and
		// ignores the new reader) if a name repeats.
		name := "page" + strconv.Itoa(i)
		opts := fpdf.ImageOptions{ImageType: imgType}
		pdf.RegisterImageOptionsReader(name, opts, bytes.NewReader(p.data))
		// Full-bleed at (0,0); flow=false keeps it absolutely positioned.
		pdf.ImageOptions(name, 0, 0, w, h, false, opts, 0, "")
	}

	// fpdf accumulates errors internally rather than returning them per call.
	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("document PDF: %w", err)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("document PDF: %w", err)
	}
	return buf.Bytes(), nil
}
