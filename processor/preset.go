package processor

// This file is intentionally tag-free: it imports no govips/libvips code, so it
// compiles and its tests run locally without libvips installed. The govips
// pipeline that consumes these presets lives in the `//go:build vips` files.

// Format is the output container/codec for a preset. It is a local type (not
// vips.ImageType) precisely so this file stays free of the govips import.
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatWebP Format = "webp"
	FormatAVIF Format = "avif"
	FormatPNG  Format = "png"
)

// Kind selects how the pipeline turns a preset into output. KindImage (the zero
// value) is the normal one-image-out path. KindFaviconPack produces a whole set
// of files (multiple PNG sizes, an .ico, a web manifest, an HTML snippet) bundled
// under a folder in the ZIP, so a single preset can yield a drop-in favicon pack.
type Kind int

const (
	KindImage Kind = iota
	KindFaviconPack
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
}

// Resizes reports whether the preset crops to fixed dimensions. When false the
// pipeline keeps the source dimensions.
func (p Preset) Resizes() bool {
	return p.Width > 0 && p.Height > 0
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

	{Name: "instagram_square", Format: FormatJPEG, Width: 1080, Height: 1080, Quality: 80, Progressive: true},
	{Name: "instagram_portrait", Format: FormatJPEG, Width: 1080, Height: 1350, Quality: 80, Progressive: true},
	{Name: "linkedin", Format: FormatJPEG, Width: 1200, Height: 627, Quality: 80, Progressive: true},
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

	// Wide JPEG banners.
	{Name: "email_header", Format: FormatJPEG, Width: 600, Height: 200, Quality: 80, Progressive: true},
	{Name: "web_banner", Format: FormatJPEG, Width: 1920, Height: 480, Quality: 80, Progressive: true},
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
