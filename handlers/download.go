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

			// Free the job once its bytes have been streamed. A second download
			// of the same id then 404s, as the acceptance criteria expect.
			store.Delete(jobID)
		})
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
