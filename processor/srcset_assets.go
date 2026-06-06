package processor

// Tag-free: these build the static text members of the srcset pack (the paste-in
// <picture> snippet and the README) plus the extension mapping the assembler uses
// for member filenames. No govips import, so they compile and unit-test locally
// without libvips. The width renders themselves are produced by the vips pipeline
// (see srcset_vips.go).

import (
	"fmt"
	"strings"
)

// srcsetStem is the fixed basename used for every member of a srcset pack
// (e.g. "image-768w.webp"). The source's own filename isn't available inside the
// processor — the handlers strip it — and the pack folder is already namespaced
// per-source in the ZIP when there are multiple uploads, so a fixed stem can't
// collide. The snippet references the same stem so it always names files that
// exist in the folder.
const srcsetStem = "image"

// srcsetExt maps an output format to the file extension used in srcset member
// names and in the generated <picture> snippet. Kept here (tag-free) so the
// assembler and the snippet generator agree on extensions and it's testable
// without libvips. Mirrors handlers.extFor, intentionally duplicated to avoid a
// cross-package dependency from the processor into the handlers.
func srcsetExt(f Format) string {
	switch f {
	case FormatJPEG:
		return "jpg"
	case FormatWebP:
		return "webp"
	case FormatAVIF:
		return "avif"
	case FormatPNG:
		return "png"
	default:
		return "bin"
	}
}

// srcsetMember returns the member filename for a given width and format, e.g.
// "image-768w.webp". Used by both the assembler (to name OutputFiles) and the
// snippet generator (to reference them) so the two never disagree.
func srcsetMember(width int, f Format) string {
	return fmt.Sprintf("%s-%dw.%s", srcsetStem, width, srcsetExt(f))
}

// srcsetSnippetBytes builds the paste-in <picture> markup naming exactly the
// widths that were rendered (the no-upscale filter may have dropped some). It
// emits one <source> per modern format (AVIF then WebP) and a JPEG <img>
// fallback whose src is the largest width. widths must be sorted ascending and
// non-empty; the largest is used for the fallback src and the intrinsic width
// attribute.
func srcsetSnippetBytes(widths []int) []byte {
	if len(widths) == 0 {
		return []byte("<!-- srcset pack: no widths rendered -->\n")
	}
	largest := widths[len(widths)-1]

	// sizes is a sensible default the user is told to tune in the README: the
	// image is full viewport width until it reaches its largest rendered width,
	// then capped there.
	sizes := fmt.Sprintf("(max-width: %dpx) 100vw, %dpx", largest, largest)

	srcset := func(f Format) string {
		parts := make([]string, 0, len(widths))
		for _, w := range widths {
			parts = append(parts, fmt.Sprintf("%s %dw", srcsetMember(w, f), w))
		}
		return strings.Join(parts, ", ")
	}

	var b strings.Builder
	b.WriteString("<picture>\n")
	b.WriteString(fmt.Sprintf("  <source type=\"image/avif\"\n          srcset=\"%s\"\n          sizes=\"%s\">\n", srcset(FormatAVIF), sizes))
	b.WriteString(fmt.Sprintf("  <source type=\"image/webp\"\n          srcset=\"%s\"\n          sizes=\"%s\">\n", srcset(FormatWebP), sizes))
	b.WriteString(fmt.Sprintf("  <img src=\"%s\"\n       srcset=\"%s\"\n       sizes=\"%s\"\n       alt=\"\" loading=\"lazy\" decoding=\"async\" width=\"%d\">\n",
		srcsetMember(largest, FormatJPEG), srcset(FormatJPEG), sizes, largest))
	b.WriteString("</picture>\n")
	return []byte(b.String())
}

// srcsetReadmeBytes returns the drop-in instruction file for the srcset pack.
func srcsetReadmeBytes() []byte {
	return []byte(srcsetReadme)
}

const srcsetReadme = `Responsive image pack — drop-in instructions
============================================

This folder contains your image rendered at several widths, each in three
formats: AVIF (smallest, modern browsers), WebP (small, very wide support), and
JPEG (the universal fallback). The browser automatically downloads the single
best file for the visitor's screen and connection.

1. Copy this whole folder into your site (e.g. /images/my-photo/) and adjust the
   paths in the snippet below to match where you put it.

2. Paste the contents of index.html into your page where the image should go.
   It is a <picture> element wired up with srcset + sizes.

3. Set a real "alt" describing the image (currently empty) for accessibility and
   SEO.

4. Tune the "sizes" attribute to how wide the image actually renders in your
   layout. It currently says "100vw up to the largest width". If your image sits
   in a column that is, say, half the page, use something like
   sizes="(max-width: 600px) 100vw, 50vw" — this lets the browser pick a smaller
   file and saves bandwidth. The widths in srcset never change; only sizes does.

Files:
   index.html        the <picture> snippet to paste
   image-<W>w.avif   each width, AVIF  (modern browsers, smallest)
   image-<W>w.webp   each width, WebP  (wide support, small)
   image-<W>w.jpg    each width, JPEG  (universal fallback)
`
