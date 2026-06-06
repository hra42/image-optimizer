package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/hra42/image-optimizer/processor"
)

// errNoPresets is returned when an upload selects no valid presets.
var errNoPresets = errors.New("no valid presets selected")

// allowedExts is the set of input extensions we accept. It backstops content
// sniffing, which does not reliably recognize AVIF or HEIC/HEIF — both are
// admitted by extension only (see allowedSniffed). HEIC/HEIF are decoded by
// libvips' heifload (libheif/libde265), so iPhone photos convert just like any
// other input.
var allowedExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".avif": true,
	".heic": true,
	".heif": true,
	".svg":  true,
}

// allowedSniffed is the set of MIME types http.DetectContentType may return for
// our supported inputs. AVIF and HEIC/HEIF are absent on purpose — the stdlib
// sniffer does not recognize them, so they are admitted by extension. SVG is
// sniffed as text/xml or text/plain (it is XML), so it too relies on extension.
var allowedSniffed = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// Upload handles POST /upload. It accepts multipart form-data with one or more
// image files (field "files", or "file") and selected preset names (field
// "presets", repeated or comma-separated), validates type and size, kicks off
// asynchronous processing, and returns the job id immediately. maxFileBytes is
// the per-file size cap (from config); oversized files are rejected with 400.
func Upload(store *Store, maxFileBytes int64) fiber.Handler {
	return func(c fiber.Ctx) error {
		form, err := c.MultipartForm()
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid multipart form")
		}

		headers := form.File["files"]
		if len(headers) == 0 {
			headers = form.File["file"]
		}
		if len(headers) == 0 {
			return fiber.NewError(fiber.StatusBadRequest, "no files uploaded")
		}

		presets, err := selectPresets(form.Value["presets"])
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}

		// Optional per-file crop anchors, indexed to upload order (see parseFocals).
		focals, err := parseFocals(form.Value["focals"], len(headers))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}

		files := make([]srcFile, 0, len(headers))
		for i, fh := range headers {
			data, err := readValidImage(fh, maxFileBytes)
			if err != nil {
				return fiber.NewError(fiber.StatusBadRequest, err.Error())
			}
			files = append(files, srcFile{
				base:  baseName(fh.Filename),
				data:  data,
				focal: focals[i],
			})
		}

		// Per-image presets run once per file; bundle presets (e.g. a document
		// PDF) consume all files at once and count as a single unit each.
		imagePresets, bundlePresets := partitionPresets(presets)
		total := len(files)*len(imagePresets) + len(bundlePresets)

		job := store.Create(total)
		// Track the goroutine so graceful shutdown can drain in-flight jobs.
		store.Go(func() { runJob(job, files, imagePresets, bundlePresets) })

		return c.JSON(fiber.Map{"jobId": job.ID})
	}
}

// partitionPresets splits resolved presets into per-image presets (run once per
// file) and bundle presets (run once over all files), preserving order within
// each group.
func partitionPresets(ps []processor.Preset) (image, bundle []processor.Preset) {
	for _, p := range ps {
		if p.IsBundle() {
			bundle = append(bundle, p)
		} else {
			image = append(image, p)
		}
	}
	return image, bundle
}

// selectPresets resolves the requested preset names (each value may itself be a
// comma-separated list) to Preset definitions, deduping and rejecting unknowns.
func selectPresets(values []string) ([]processor.Preset, error) {
	seen := make(map[string]bool)
	var out []processor.Preset
	for _, v := range values {
		for _, name := range strings.Split(v, ",") {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			p, ok := processor.PresetByName(name)
			if !ok {
				return nil, fmt.Errorf("unknown preset %q", name)
			}
			seen[name] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, errNoPresets
	}
	return out, nil
}

// parseFocals resolves the optional "focals" form value into one FocalPoint per
// uploaded file, indexed to upload order. The value (if present) is a single JSON
// array parallel to the files: each element is {"x":..,"y":..} for a user-adjusted
// crop or null to keep the default attention crop. n is the file count.
//
// Absent/empty focals are fine (no file was adjusted) and yield all-unset points.
// A present array whose length doesn't match the file count is rejected, so a
// focal can never be silently misattributed to the wrong file. Coordinates are
// clamped to [0,1] rather than rejected — the UI derives them from pointer
// positions and small float drift past the edge shouldn't fail the whole upload.
func parseFocals(values []string, n int) ([]processor.FocalPoint, error) {
	out := make([]processor.FocalPoint, n) // zero value = {Set: false}
	if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return out, nil
	}
	var raw []*struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := json.Unmarshal([]byte(values[0]), &raw); err != nil {
		return nil, errors.New("invalid focals: expected a JSON array")
	}
	if len(raw) != n {
		return nil, fmt.Errorf("focals length %d does not match %d files", len(raw), n)
	}
	for i, r := range raw {
		if r == nil {
			continue // null → keep the default attention crop for this file
		}
		out[i] = processor.FocalPoint{X: clamp01(r.X), Y: clamp01(r.Y), Set: true}
	}
	return out, nil
}

// clamp01 constrains v to the [0,1] normalized range.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// readValidImage enforces the size cap, reads the file into memory, and verifies
// it is a supported image by content sniff OR extension (AVIF relies on the
// latter, as the stdlib sniffer does not recognize it).
func readValidImage(fh *multipart.FileHeader, maxFileBytes int64) ([]byte, error) {
	if fh.Size > maxFileBytes {
		return nil, fmt.Errorf("%q exceeds the %dMB limit", fh.Filename, maxFileBytes>>20)
	}
	f, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("cannot read %q", fh.Filename)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("cannot read %q", fh.Filename)
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	sniff := http.DetectContentType(data) // reads up to the first 512 bytes
	if !allowedSniffed[sniff] && !allowedExts[ext] {
		return nil, fmt.Errorf("%q is not a supported image (JPEG, PNG, WebP, AVIF, HEIC, SVG)", fh.Filename)
	}
	return data, nil
}

// baseName returns the filename without directory or extension, used to
// namespace ZIP entries when more than one file is uploaded.
func baseName(name string) string {
	base := filepath.Base(name)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
