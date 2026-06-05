package main

import (
	"embed"
	"io/fs"
	"log"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/hra42/image-optimizer/handlers"
	"github.com/hra42/image-optimizer/processor"
)

// frontendDist embeds the compiled Svelte SPA. The Docker frontend stage
// produces frontend/dist; for local builds a stub index.html keeps this
// directive satisfied. See README.md.
//
//go:embed all:frontend/dist
var frontendDist embed.FS

// maxUploadBytes caps the whole multipart request body. Fiber v3 defaults to
// 4 MB; uploads carry up to several 50 MB images, so the limit is raised to give
// headroom for multiple files plus multipart boundaries. Per-file 50 MB
// enforcement lives in the upload handler.
const maxUploadBytes = 64 << 20

func main() {
	app := fiber.New(fiber.Config{
		BodyLimit: maxUploadBytes,
	})

	// Initialize libvips (real config in vips builds; no-op otherwise).
	if err := processor.Startup(); err != nil {
		log.Fatalf("processor startup: %v", err)
	}
	defer processor.Shutdown()

	// API routes (health, upload, progress SSE, download). Registered before the
	// SPA catch-all so they are not swallowed by the wildcard route.
	handlers.RegisterRoutes(app)

	// Serve the embedded Svelte SPA at the root.
	dist, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		log.Fatalf("failed to scope embedded frontend: %v", err)
	}
	app.Get("/*", static.New("", static.Config{
		FS:     dist,
		Browse: false,
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
