# Image Optimizer

High-performance web image optimizer with multi-target presets (web, Instagram,
LinkedIn, Twitter/X, OG images) built on **Fiber v3** + **govips (libvips)**.

Accepts image uploads and outputs optimized variants for multiple targets. The
Svelte SPA is compiled to static files and embedded directly into the Go binary,
so the whole app ships as a single Docker container.

## Tech stack

- **Go + Fiber v3** — single compiled binary, low memory footprint
- **govips / libvips** — the image processing engine
- **Svelte + Vite** — zero-runtime SPA, embedded via `//go:embed`
- **Single Docker container** — Fiber serves both the API and the SPA

## Project layout

```
.
├── main.go            # Fiber app, /health, embeds frontend/dist
├── handlers/          # HTTP handlers (health, and later: upload/progress/download)
├── processor/         # govips pipeline (HRA-163)
├── frontend/          # Svelte + Vite app; dist/ is the go:embed target
├── Dockerfile         # 3-stage build: Vite → Go (cgo+libvips) → debian-slim
└── docker-compose.yml # local dev with hot-reload
```

> **Note on `frontend/dist`:** a small stub `index.html` is committed so
> `go build` works locally without first running Vite. The Docker frontend
> stage overwrites it with a real Vite build. The real UI lands in HRA-165.

## Development

Hot-reload for both backend (Air) and frontend (Vite dev server):

```sh
docker compose up
```

- Backend: http://localhost:3000 (`/health` returns `{"status":"ok"}`)
- Frontend dev server: http://localhost:5173

## Production image

The single self-contained image (Vite build embedded into the Go binary):

```sh
docker build -t image-optimizer .
docker run -p 3000:3000 image-optimizer
curl http://localhost:3000/health   # -> 200 {"status":"ok"}
```

## Requirements

- Go 1.25+ · Node 22+ · Docker
- libvips ≥ 8.14 (provided inside the Docker images; install locally only if
  building the Go binary outside Docker)
