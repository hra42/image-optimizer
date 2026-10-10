package handlers

import "github.com/gofiber/fiber/v3"

// RegisterRoutes wires every API route onto the app, sharing one in-memory job
// store across upload, progress, and download. maxFileBytes is the per-file
// upload cap; version is the build version surfaced at /version (and in the UI
// footer). bgModelBaseURL is surfaced at /config so the SPA knows where to load
// background-removal models from (empty disables the feature). It returns the
// store so the caller can start its TTL reaper and drain in-flight jobs on
// shutdown. Call this before the SPA catch-all so the API paths
// are not swallowed by the wildcard route.
func RegisterRoutes(app *fiber.App, maxFileBytes int64, version, bgModelBaseURL string) *Store {
	store := NewStore()

	app.Get("/health", Health)
	app.Get("/version", Version(version))
	app.Get("/config", ClientConfig(maxFileBytes, bgModelBaseURL))
	app.Post("/upload", Upload(store, maxFileBytes))
	app.Get("/progress/:jobId", Progress(store))
	app.Get("/download/:jobId", Download(store))

	return store
}
