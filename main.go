package main

import (
	"context"
	"embed"
	"flag"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/hra42/image-optimizer/config"
	"github.com/hra42/image-optimizer/handlers"
	"github.com/hra42/image-optimizer/processor"
)

// frontendDist embeds the compiled Svelte SPA. The Docker frontend stage
// produces frontend/dist; for local builds a stub index.html keeps this
// directive satisfied. See README.md.
//
//go:embed all:frontend/dist
var frontendDist embed.FS

// shutdownGrace bounds each phase of graceful shutdown: how long Fiber waits for
// open connections to close, and how long we wait for in-flight jobs to drain.
const shutdownGrace = 30 * time.Second

// version is the build version, surfaced at /version and in the UI footer. It is
// injected at build time from the git tag via:
//
//	go build -ldflags "-X main.version=$(git describe --tags)"
//
// (the Dockerfile passes this through a build arg). Local builds keep "dev".
var version = "dev"

func main() {
	// -healthcheck turns the binary into its own health probe (used by the
	// Docker HEALTHCHECK so the minimal runtime image needs no curl/wget).
	healthcheck := flag.Bool("healthcheck", false, "probe GET /health and exit 0/1")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthcheck())
	}

	cfg := config.Load()

	app := fiber.New(fiber.Config{
		BodyLimit: int(cfg.MaxUploadBytes),
	})

	// Initialize libvips (real config in vips builds; no-op otherwise). Worker
	// count is configurable via WORKER_COUNT; <=0 falls back to NumCPU.
	if err := processor.Startup(cfg.Workers); err != nil {
		log.Fatalf("processor startup: %v", err)
	}

	// Root context canceled on SIGINT/SIGTERM; drives the reaper and shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// API routes (health, upload, progress SSE, download). Registered before the
	// SPA catch-all so they are not swallowed by the wildcard route. The returned
	// store owns job lifecycle; start its TTL reaper and drain it on shutdown.
	store := handlers.RegisterRoutes(app, cfg.MaxFileBytes, version)
	store.StartReaper(ctx, cfg.JobTTL)

	// Serve the embedded Svelte SPA at the root.
	dist, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		log.Fatalf("failed to scope embedded frontend: %v", err)
	}
	// Go's default MIME table has no entry for .webmanifest, so the static
	// middleware would serve site.webmanifest as text/plain. Register the spec
	// type so browsers accept it as a PWA manifest.
	if err := mime.AddExtensionType(".webmanifest", "application/manifest+json"); err != nil {
		log.Printf("warning: could not register .webmanifest MIME type: %v", err)
	}
	app.Get("/*", static.New("", static.Config{
		FS:     dist,
		Browse: false,
		// Cache-Control is the contract that keeps a hashed-asset SPA consistent
		// behind a CDN (Cloudflare here). Vite fingerprints everything under
		// /assets/ (index-<hash>.js/.css), so those are immutable and safe to
		// cache forever — the URL changes whenever the content does. index.html
		// (and any deep-link path that falls back to it) references those hashes
		// and must NEVER be cached, or a redeploy leaves the CDN serving an old
		// HTML shell pointing at asset hashes the origin no longer has (404 +
		// "MIME type text/plain" style failures). ModifyResponse runs after the
		// middleware's default header and only on successful responses.
		ModifyResponse: func(c fiber.Ctx) error {
			if strings.HasPrefix(c.Path(), "/assets/") {
				c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
			} else {
				c.Set(fiber.HeaderCacheControl, "no-cache")
			}
			return nil
		},
	}))

	// Serve in a goroutine so main can wait for a shutdown signal.
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("listening on :%s", cfg.Port)
		serveErr <- app.Listen(":" + cfg.Port)
	}()

	select {
	case err := <-serveErr:
		// Listener failed to start (e.g. port in use).
		processor.Shutdown()
		log.Fatalf("server error: %v", err)
	case <-ctx.Done():
		log.Println("shutdown: signal received, draining")
	}

	// 1. Stop accepting new connections, letting open ones finish.
	if err := app.ShutdownWithTimeout(shutdownGrace); err != nil {
		log.Printf("shutdown: server stop: %v", err)
	}

	// 2. Wait for in-flight jobs to finish processing (bounded) so their SSE
	//    clients get a terminal event and the ZIP stays briefly downloadable.
	if waitWithTimeout(store.Wait, shutdownGrace) {
		log.Println("shutdown: in-flight jobs drained")
	} else {
		log.Printf("shutdown: drain timed out after %s, exiting anyway", shutdownGrace)
	}

	// 3. Tear down libvips.
	processor.Shutdown()
	log.Println("shutdown: complete")
}

// waitWithTimeout runs wait (a blocking call) and reports whether it returned
// before the timeout.
func waitWithTimeout(wait func(), timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// runHealthcheck performs a single GET /health against the local server and
// returns a process exit code (0 healthy, 1 otherwise). It honors PORT so it
// targets the same port the server listens on.
func runHealthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		log.Printf("healthcheck: %v", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("healthcheck: status %d", resp.StatusCode)
		return 1
	}
	return 0
}
