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

// Preset describes one output variant: target format, dimensions, and the
// format-specific encoding knobs. A zero Width/Height means "keep the source
// dimensions" (used by the website_* presets).
type Preset struct {
	Name        string
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

// Result is one preset's output. Per-preset failures are carried in Err so a
// single bad preset (e.g. an unsupported source) doesn't sink the others.
type Result struct {
	Preset Preset
	Data   []byte // encoded bytes, ready to stream or zip
	Width  int    // actual output dimensions
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
	{Name: "instagram_square", Format: FormatJPEG, Width: 1080, Height: 1080, Quality: 80, Progressive: true},
	{Name: "instagram_portrait", Format: FormatJPEG, Width: 1080, Height: 1350, Quality: 80, Progressive: true},
	{Name: "linkedin", Format: FormatJPEG, Width: 1200, Height: 627, Quality: 80, Progressive: true},
	{Name: "twitter", Format: FormatJPEG, Width: 1200, Height: 675, Quality: 80, Progressive: true},
	{Name: "og_image", Format: FormatPNG, Width: 1200, Height: 630, Compression: 6},
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
