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
├── main.go            # Fiber app, config, graceful shutdown, embeds frontend/dist
├── config/            # env-var configuration (port, limits, workers, job TTL)
├── handlers/          # HTTP handlers: health, upload, progress (SSE), download
├── processor/         # govips pipeline, worker semaphore, presets
├── frontend/          # Svelte + Vite app; dist/ is the go:embed target
├── Dockerfile         # 3-stage build: Vite → Go (cgo+libvips) → debian-slim
└── docker-compose.yml # local dev with hot-reload
```

> **Note on `frontend/dist`:** a small stub `index.html` is committed so
> `go build` works locally without first running Vite. The Docker frontend
> stage overwrites it with the real Vite build.

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

The final image contains only the Go binary plus the libvips runtime libs
(`libvips42`) on `debian:bookworm-slim`, and runs as a non-root user.

### Health check

The image ships a Docker `HEALTHCHECK` that probes `GET /health`. The binary
self-probes via its `-healthcheck` flag, so no `curl`/`wget` is needed in the
minimal runtime image:

```sh
docker run -d -p 3000:3000 image-optimizer
docker ps   # STATUS shows "healthy" once the start period elapses
```

### Graceful shutdown

On `SIGINT`/`SIGTERM` the server stops accepting new connections, drains
in-flight jobs (so their SSE clients receive a terminal event and the ZIP stays
briefly downloadable), tears down libvips, then exits. Each phase is bounded by a
30s timeout so shutdown never hangs.

### Job lifetime

Job state lives in memory only — there is no disk temp storage. A job is freed
when its ZIP is downloaded, or by a background reaper after `JOB_TTL_MINUTES`
(default 10), whichever comes first. This bounds memory for jobs that are never
downloaded.

## Environment variables

All configuration is via environment variables, read once at startup (the
resolved values are logged). Invalid or non-positive numeric values fall back to
the default rather than failing startup.

| Variable           | Default          | Description                                                        |
| ------------------ | ---------------- | ------------------------------------------------------------------ |
| `PORT`             | `3000`           | TCP port the HTTP server listens on.                               |
| `MAX_FILE_SIZE_MB` | `50`             | Per-file upload cap. Larger files are rejected with `400`.         |
| `WORKER_COUNT`     | number of CPUs   | Max concurrent libvips pipelines (the real concurrency limit).     |
| `JOB_TTL_MINUTES`  | `10`             | How long a job's in-memory state is retained before the reaper frees it. |

The whole-request multipart body limit is derived from `MAX_FILE_SIZE_MB` plus
headroom for multiple files and multipart boundaries.

## Requirements

- Go 1.26+ · Node 22+ · Docker
- libvips ≥ 8.14 (provided inside the Docker images; install locally only if
  building the Go binary outside Docker)
