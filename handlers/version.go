package handlers

import "github.com/gofiber/fiber/v3"

// Version returns a handler that responds 200 with the build version as JSON, so
// the frontend can show which release it is running. The value is injected at
// build time (git tag via -ldflags); it falls back to "dev" for local builds.
func Version(version string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"version": version})
	}
}
