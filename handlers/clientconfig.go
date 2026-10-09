package handlers

import "github.com/gofiber/fiber/v3"

// ClientConfig returns a handler exposing the runtime settings the SPA needs:
// the per-file upload cap (so the background remover can warn before an
// oversized cutout fails the upload) and the background-removal model base URL
// (empty disables the feature in the UI).
func ClientConfig(maxFileBytes int64, bgModelBaseURL string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"maxFileBytes": maxFileBytes,
			"bgRemoval":    fiber.Map{"baseUrl": bgModelBaseURL},
		})
	}
}
