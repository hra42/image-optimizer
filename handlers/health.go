// Package handlers contains the Fiber HTTP handlers for the image optimizer.
package handlers

import "github.com/gofiber/fiber/v3"

// Health responds 200 with a small JSON body so Docker, compose, and load
// balancers can verify the server is up.
func Health(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
