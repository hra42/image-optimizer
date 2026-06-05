package main

import (
	"embed"
	"io/fs"
	"log"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/hra42/image-optimizer/handlers"
)

// frontendDist embeds the compiled Svelte SPA. The Docker frontend stage
// produces frontend/dist; for local builds a stub index.html keeps this
// directive satisfied. See README.md.
//
//go:embed all:frontend/dist
var frontendDist embed.FS

func main() {
	app := fiber.New()

	// Health check — used by Docker and load balancers.
	app.Get("/health", handlers.Health)

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
