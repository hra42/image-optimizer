package processor

// This file is intentionally tag-free: it imports no govips/libvips code, so it
// compiles and its tests run locally without libvips installed. The govips
// pipeline that consumes these presets lives in the `//go:build vips` files.

import "errors"

// ErrVipsNotBuilt is returned (directly, or via Result.Err for ProcessBundle)
// when the binary was built without the `vips` tag, i.e. without libvips support
// compiled in. Declared here (tag-free) so both the real and stub pipelines —
// and the handlers that branch on it — reference the same value in every build.
var ErrVipsNotBuilt = errors.New("processor: built without 'vips' tag; libvips unavailable")

// Format is the output container/codec for a preset. It is a local type (not
// vips.ImageType) precisely so this file stays free of the govips import.
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatWebP Format = "webp"
	FormatAVIF Format = "avif"
	FormatPNG  Format = "png"

	// FormatAuto is a sentinel meaning "keep the source image's format". It is
	// never exported directly: the vips pipeline detects the input format and
	// resolves it to one of the concrete formats above (see resolveFormat in
	// pipeline.go), then writes the resolved Format back onto the Result's preset
	// so the download handler picks the right extension. Used by the compress_*
	// presets, which only re-encode and must not change the file type.
	FormatAuto Format = "auto"
)

// CompressTier selects an encoding aggressiveness for the compress_* presets.
// These presets keep the source format and dimensions and only tune the codec
// knobs; the per-format knob values for each tier are resolved in the pipeline
// (see compressParams in pipeline.go), not stored on the registry row, because
// they depend on the format that FormatAuto resolves to. TierNone (the zero
// value) means "this is not a compress preset" — every existing preset is
// unaffected.
type CompressTier int

const (
	TierNone CompressTier = iota
	TierBest
	TierBalanced
	TierMax
)

// Kind selects how the pipeline turns a preset into output. KindImage (the zero
// value) is the normal one-image-out path. KindFaviconPack produces a whole set
// of files (multiple PNG sizes, an .ico, a web manifest, an HTML snippet) bundled
// under a folder in the ZIP, so a single preset can yield a drop-in favicon pack.
// KindSrcsetPack is likewise a per-file pack: one source becomes a folder of
// widths, each in AVIF + WebP + JPEG, plus a paste-in <picture> snippet and a
// README. KindDocumentPDF is a *bundle* kind: it consumes ALL uploaded files at
// once and emits one multi-page PDF (each image a page, in upload order) — a
// LinkedIn document/carousel post. Unlike the other kinds it is not per-file.
type Kind int

const (
	KindImage Kind = iota
	KindFaviconPack
	KindDocumentPDF
	KindSrcsetPack
)

// Preset describes one output variant: target format, dimensions, and the
// format-specific encoding knobs. A zero Width/Height means "keep the source
// dimensions" (used by the website_* presets).
type Preset struct {
	Name        string
	Kind        Kind // KindImage (default) or a multi-file pack
	Format      Format
	Width       int // 0 = keep original
	Height      int // 0 = keep original
	Quality     int
	Progressive bool // JPEG interlace
	Effort      int  // AVIF Effort / WebP ReductionEffort
	Compression int  // PNG compression (0–9)

	// Tier, when non-zero, marks a compress preset: the pipeline resolves
	// FormatAuto to the input format and applies tier-tuned codec knobs instead
	// of the (empty) Quality/Compression/Effort fields above.
	Tier CompressTier
}

// Resizes reports whether the preset crops to fixed dimensions. When false the
// pipeline keeps the source dimensions.
func (p Preset) Resizes() bool {
	return p.Width > 0 && p.Height > 0
}

// IsBundle reports whether the preset consumes ALL uploaded files at once and
// produces a single output for the whole job (rather than running per-file).
// Bundle presets take a separate path through the orchestrator (see runJob's
// bundle phase) and their output is a top-level ZIP entry, not namespaced per
// source. Mirror of the frontend BUNDLE_PRESETS set in frontend/src/lib/presets.js.
func (p Preset) IsBundle() bool {
	return p.Kind == KindDocumentPDF
}

// OutputFile is one named file inside a multi-file preset (e.g. a favicon pack
// member). Name is the path relative to the preset's folder in the ZIP.
type OutputFile struct {
	Name string
	Data []byte
}

// Result is one preset's output. Per-preset failures are carried in Err so a
// single bad preset (e.g. an unsupported source) doesn't sink the others.
//
// A KindImage preset fills Data (single encoded image) and leaves Files nil. A
// pack preset (e.g. KindFaviconPack) fills Files and leaves Data nil. Exactly
// one of the two is populated on success.
type Result struct {
	Preset Preset
	Data   []byte       // single-image output (KindImage)
	Files  []OutputFile // multi-file output (pack presets)
	Width  int          // actual output dimensions (KindImage)
	Height int
	Err    error
}

// ResultFunc is invoked once per preset as soon as that preset finishes, on the
// goroutine that produced it. Because presets complete in parallel it must be
// safe for concurrent calls, and because it runs while holding a worker slot it
// must not block for long. i is the preset's index in the input slice.
type ResultFunc func(i int, r Result)

// presets is the canonical registry, defined once in spec order.
var presets = []Preset{
	{Name: "website_webp", Format: FormatWebP, Quality: 80},
	{Name: "website_avif", Format: FormatAVIF, Quality: 60, Effort: 4},

	// Full-size JPEG/PNG: optimize and strip metadata without resizing. The
	// JPEG/PNG counterparts to website_webp/website_avif.
	{Name: "jpeg_original", Format: FormatJPEG, Quality: 80, Progressive: true},
	{Name: "png_original", Format: FormatPNG, Compression: 6},

	// Convert: faithful, high-quality format conversion at the original size —
	// the "just turn my iPhone HEIC into a usable file" path. Higher quality
	// than the web-optimized website_* presets so the output stays close to the
	// source. No resize/crop.
	{Name: "convert_jpeg", Format: FormatJPEG, Quality: 92, Progressive: true},
	{Name: "convert_png", Format: FormatPNG, Compression: 6},
	{Name: "convert_webp", Format: FormatWebP, Quality: 90},
	{Name: "convert_avif", Format: FormatAVIF, Quality: 80, Effort: 4},

	// Compress: "just make this smaller for the web" — keep the source format
	// (FormatAuto) and original dimensions, tune only the encoding across three
	// honest tiers. The per-format knobs for each tier live in compressParams
	// (pipeline.go), since they depend on the format FormatAuto resolves to.
	{Name: "compress_best", Format: FormatAuto, Tier: TierBest},
	{Name: "compress_balanced", Format: FormatAuto, Tier: TierBalanced},
	{Name: "compress_max", Format: FormatAuto, Tier: TierMax},

	{Name: "instagram_square", Format: FormatJPEG, Width: 1080, Height: 1080, Quality: 80, Progressive: true},
	{Name: "instagram_portrait", Format: FormatJPEG, Width: 1080, Height: 1350, Quality: 80, Progressive: true},
	{Name: "instagram_story", Format: FormatJPEG, Width: 1080, Height: 1920, Quality: 80, Progressive: true},
	{Name: "linkedin", Format: FormatJPEG, Width: 1200, Height: 627, Quality: 80, Progressive: true},
	{Name: "linkedin_profile_banner", Format: FormatJPEG, Width: 1584, Height: 396, Quality: 80, Progressive: true},
	{Name: "linkedin_company_banner", Format: FormatJPEG, Width: 1128, Height: 191, Quality: 80, Progressive: true},
	{Name: "twitter", Format: FormatJPEG, Width: 1200, Height: 675, Quality: 80, Progressive: true},
	{Name: "facebook_post", Format: FormatJPEG, Width: 1200, Height: 630, Quality: 80, Progressive: true},
	{Name: "pinterest_pin", Format: FormatJPEG, Width: 1000, Height: 1500, Quality: 80, Progressive: true},

	{Name: "og_image", Format: FormatPNG, Width: 1200, Height: 630, Compression: 6},

	// Favicon pack: a full drop-in icon set (multiple PNG sizes, favicon.ico,
	// apple-touch-icon, web manifest, and an HTML snippet) generated from one
	// center-cropped square source. Width/Height are unused — the pack defines
	// its own sizes (see faviconSizes in favicon_vips.go).
	{Name: "favicon", Kind: KindFaviconPack, Format: FormatPNG, Compression: 6},
	// thumbnail stays a single square PNG for anyone who just wants one icon.
	{Name: "thumbnail", Format: FormatPNG, Width: 400, Height: 400, Compression: 6},

	// Responsive srcset pack: one source → a folder of widths each in AVIF + WebP
	// + JPEG, plus a paste-in <picture> snippet and a README. Width/Height are
	// unused — the pack defines its own widths (see srcsetWidths in
	// srcset_vips.go). Format is a placeholder; the assembler emits all three
	// formats. Quality/Effort/Progressive are the master web-optimized knobs the
	// assembler applies per format (matching the website_* defaults).
	{Name: "srcset_web", Kind: KindSrcsetPack, Format: FormatWebP, Quality: 80, Effort: 4, Progressive: true},

	// Wide JPEG banners.
	{Name: "email_header", Format: FormatJPEG, Width: 600, Height: 200, Quality: 80, Progressive: true},
	{Name: "web_banner", Format: FormatJPEG, Width: 1920, Height: 480, Quality: 80, Progressive: true},

	// Document posts: bundle presets (KindDocumentPDF) — every uploaded image
	// becomes one page of a single multi-page PDF, in upload order. This is the
	// LinkedIn "document post" / carousel shape. Width/Height are the per-page
	// box each image is centre-cropped to; Format must be JPEG (the only codec
	// the PDF assembler embeds, see processor/pdf.go).
	{Name: "linkedin_doc_portrait", Kind: KindDocumentPDF, Format: FormatJPEG, Width: 1080, Height: 1350, Quality: 85, Progressive: true},
	{Name: "linkedin_doc_square", Kind: KindDocumentPDF, Format: FormatJPEG, Width: 1080, Height: 1080, Quality: 85, Progressive: true},
}

// AllPresets returns a copy of the full preset registry.
func AllPresets() []Preset {
	out := make([]Preset, len(presets))
	copy(out, presets)
	return out
}

// PresetByName looks up a preset by its registry name.
func PresetByName(name string) (Preset, bool) {
	for _, p := range presets {
		if p.Name == name {
			return p, true
		}
	}
	return Preset{}, false
}
