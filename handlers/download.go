package handlers

import (
	"archive/zip"
	"bufio"
	"path"

	"github.com/gofiber/fiber/v3"

	"github.com/hra42/image-optimizer/processor"
)

// Download handles GET /download/:jobId. It streams a ZIP of every successful
// output for the job, then deletes the job so its in-memory state is freed.
func Download(store *Store) fiber.Handler {
	return func(c fiber.Ctx) error {
		jobID := c.Params("jobId")

		job, ok := store.Get(jobID)
		if !ok || !job.Finished() {
			return fiber.NewError(fiber.StatusNotFound, "job not found")
		}

		outputs := job.Outputs()

		// Single-file fast path: when the job produced exactly one output file,
		// return it raw (with its real content-type and filename) instead of a
		// one-entry ZIP — the common "one image, one preset" case. A pack (favicon
		// has many members) or bundle, or any job with 2+ outputs, still ZIPs.
		if of, ok := soleImageOutput(outputs); ok {
			c.Attachment(of.preset + extFor(of.format))
			c.Type(extFor(of.format)) // sets Content-Type from the extension
			freeIfRedownloaded(store, job, jobID)
			return c.Send(of.data)
		}

		// Content-Disposition: attachment; filename="optimized.zip".
		c.Attachment("optimized.zip")

		// Whether to namespace entries by source filename depends on how many
		// distinct sources contributed outputs.
		multiSource := hasMultipleSources(outputs)

		return c.SendStreamWriter(func(w *bufio.Writer) {
			zw := zip.NewWriter(w)
			for _, of := range outputs {
				// Bundle outputs (e.g. a document PDF) are written at the ZIP
				// root using the member's own filename — never namespaced by
				// source or preset folder.
				if of.bundle {
					if err := writeBundle(zw, of); err != nil {
						break
					}
					continue
				}
				// Pack presets (e.g. favicon) expand into a folder of members;
				// normal presets are a single named entry.
				if len(of.pack) > 0 {
					if err := writePack(zw, of, multiSource); err != nil {
						break
					}
					continue
				}
				name := of.preset + extFor(of.format)
				if multiSource {
					name = path.Join(of.srcBase, name)
				}
				fw, err := zw.Create(name)
				if err != nil {
					break
				}
				if _, err := fw.Write(of.data); err != nil {
					break
				}
			}
			_ = zw.Close()
			_ = w.Flush()

			// The first download (typically the auto-download) keeps the job so the
			// manual button still works; the second frees it. See freeIfRedownloaded.
			freeIfRedownloaded(store, job, jobID)
		})
	}
}

// freeIfRedownloaded records a download and deletes the job once it has been
// downloaded twice. The frontend auto-downloads on completion and also shows a
// manual button as a fallback; keeping the job alive through the first download
// means a fallback click still succeeds, while the second click (or a retry)
// cleans up the in-memory state. The TTL reaper frees jobs that are never
// downloaded a second time.
func freeIfRedownloaded(store *Store, job *Job, jobID string) {
	if job.MarkDownloaded() >= 2 {
		store.Delete(jobID)
	}
}

// writePack writes every member of a pack preset into the ZIP under a folder
// named after the preset (e.g. "favicon/favicon.ico"). When the job has multiple
// source files the folder is further namespaced by source base, mirroring the
// single-file path. Any write error aborts the pack.
func writePack(zw *zip.Writer, of outFile, multiSource bool) error {
	dir := of.preset
	if multiSource {
		dir = path.Join(of.srcBase, of.preset)
	}
	for _, member := range of.pack {
		fw, err := zw.Create(path.Join(dir, member.Name))
		if err != nil {
			return err
		}
		if _, err := fw.Write(member.Data); err != nil {
			return err
		}
	}
	return nil
}

// writeBundle writes a bundle output's single member at the ZIP root, using the
// member's own filename verbatim (e.g. "linkedin_doc_portrait.pdf"). Bundle
// outputs are job-wide, so they are never namespaced by source. Any write error
// aborts the bundle.
func writeBundle(zw *zip.Writer, of outFile) error {
	for _, member := range of.pack {
		fw, err := zw.Create(member.Name)
		if err != nil {
			return err
		}
		if _, err := fw.Write(member.Data); err != nil {
			return err
		}
	}
	return nil
}

// hasMultipleSources reports whether the outputs came from more than one source
// file, which determines whether ZIP entries are namespaced by source. Bundle
// outputs are job-wide (empty srcBase) and are excluded — otherwise their empty
// source would always register as a distinct source and force namespacing.
func hasMultipleSources(outputs []outFile) bool {
	var first string
	seen := false
	for _, of := range outputs {
		if of.bundle {
			continue
		}
		if !seen {
			first = of.srcBase
			seen = true
			continue
		}
		if of.srcBase != first {
			return true
		}
	}
	return false
}

// soleImageOutput returns the single output and true when the job produced
// exactly one plain image file — one output, with image data and no pack/bundle
// members. That is the case we can serve raw instead of zipping. Packs (favicon,
// srcset) and bundles (PDF) carry their files in `pack`, so they never qualify
// even when they are the only output, since they are inherently multi-file or
// folder-shaped.
func soleImageOutput(outputs []outFile) (outFile, bool) {
	if len(outputs) != 1 {
		return outFile{}, false
	}
	of := outputs[0]
	if of.bundle || len(of.pack) > 0 || of.data == nil {
		return outFile{}, false
	}
	return of, true
}

// extFor maps an output format to its file extension.
func extFor(f processor.Format) string {
	switch f {
	case processor.FormatJPEG:
		return ".jpg"
	case processor.FormatWebP:
		return ".webp"
	case processor.FormatPNG:
		return ".png"
	case processor.FormatAVIF:
		return ".avif"
	default:
		return ".bin"
	}
}
