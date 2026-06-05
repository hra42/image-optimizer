package handlers

import "github.com/gofiber/fiber/v3"

// RegisterRoutes wires every API route onto the app, sharing one in-memory job
// store across upload, progress, and download. Call this before the SPA
// catch-all so the API paths are not swallowed by the wildcard route.
func RegisterRoutes(app *fiber.App) {
	store := NewStore()

	app.Get("/health", Health)
	app.Post("/upload", Upload(store))
	app.Get("/progress/:jobId", Progress(store))
	app.Get("/download/:jobId", Download(store))
}
