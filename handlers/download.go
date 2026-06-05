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

// hasMultipleSources reports whether the outputs came from more than one source
// file, which determines whether ZIP entries are namespaced by source.
func hasMultipleSources(outputs []outFile) bool {
	if len(outputs) == 0 {
		return false
	}
	first := outputs[0].srcBase
	for _, of := range outputs[1:] {
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
