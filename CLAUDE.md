# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A self-hostable web app that turns one uploaded image into many sizes/formats
(web formats, social presets, a full favicon pack) and returns them as a single
ZIP. Go + Fiber v3 backend with a govips/libvips processing pipeline; Svelte +
Vite SPA embedded into the Go binary via `//go:embed`. Ships as one Docker image.
Images are processed in memory only — never written to disk.

## The `vips` build tag — read this first

Image processing is gated behind the `vips` build tag, and **local builds/tests
omit it on purpose** (libvips dev headers only exist in the Docker builder stage).
This split is the single most important thing to understand here:

- `//go:build vips` files (`processor/process.go`, `pipeline.go`, `vips.go`,
  `favicon_vips.go`, plus the `*_vips_test.go` files) hold all govips/libvips code.
- `//go:build !vips` files (`processor/process_stub.go`) provide the same exported
  surface so `main.go` links in both modes. In stub mode `Startup`/`Shutdown` are
  no-ops and `Process`/`ProcessStream` return `ErrVipsNotBuilt`.
- **Tag-free files must stay tag-free**: `config/`, `handlers/`, and
  `processor/preset.go` / `ico.go` import no govips, so they compile and test
  locally. `Format`/`Kind` are local types (not `vips.ImageType`) specifically to
  keep `preset.go` import-clean. Do not add a govips import to these.

Consequence: a plain `go build` or `docker compose up` runs the backend in stub
mode — it serves `/health` and the SPA but **cannot process images** (uploads fail
the job). To exercise real processing you need the `vips` tag *and* libvips
installed, which in practice means the full `docker build` (production image).

## Commands

Local (no libvips — stub mode, good for non-processing work and most tests):
```sh
go build ./...                 # builds in stub mode (!vips)
go test ./...                  # runs tag-free tests (config, handlers, preset, ico)
go test ./processor -run TestX # single test by name
go vet ./...
```

Run the vips-tagged tests (requires libvips + headers installed):
```sh
go test -tags vips ./...
go build -tags vips -o image-optimizer .
```

Frontend (in `frontend/`):
```sh
npm ci && npm run build        # produces frontend/dist (the go:embed target)
npm run dev                    # Vite dev server with HMR on :5173
```

Hot-reload dev (backend via Air on :3000, frontend via Vite on :5173):
```sh
docker compose up
```
Note: the Air build (`.air.toml`) does **not** pass `-tags vips`, so the
hot-reload backend is also in stub mode. Use it for API wiring / SPA work, not for
testing actual image conversion.

Production image (the only way to run real end-to-end processing easily):
```sh
docker build -t image-optimizer .
docker run -p 3000:3000 image-optimizer
curl http://localhost:3000/health   # -> 200 {"status":"ok"}
```

## Architecture

Request flow: **upload → async job → SSE progress → ZIP download**, all backed by
an in-memory store with no disk or external services.

- **`main.go`** — builds the Fiber app, calls `processor.Startup(workers)`,
  registers API routes *before* the SPA catch-all (`/*`), embeds `frontend/dist`,
  and handles signal-driven graceful shutdown (stop accepting → drain in-flight
  jobs → tear down libvips, each phase bounded by `shutdownGrace` = 30s). The
  `-healthcheck` flag turns the binary into its own health probe (used by the
  Docker `HEALTHCHECK` so the minimal runtime image needs no curl/wget).

- **`handlers/store.go`** — the heart of the concurrency model. `Store` is an
  in-memory `map[id]*Job`; jobs live until their ZIP is downloaded or the TTL
  reaper frees them (`StartReaper`). **The upload→subscribe race is the subtle
  part**: `Job` keeps an append-only `events` history plus subscriber channels,
  both guarded by `j.mu`. `Subscribe` snapshots history and registers the channel
  under one lock, so every event is observed exactly once (already in history, or
  arrives on the channel after registration — never both/neither). This handles
  the client opening its `EventSource` only after the upload response already
  returned. `runJob` drives one upload through every preset, publishing a progress
  event per completed `(file, preset)` unit.

- **`handlers/upload.go`** — validates input (size cap + type via content-sniff OR
  extension; AVIF/HEIC are admitted by **extension only** because the stdlib
  sniffer doesn't recognize them), resolves selected preset names, then returns a
  `jobId` immediately and processes asynchronously via `store.Go`.

- **`processor/process.go`** (`vips` build) — the imgproxy-style worker pool: two
  semaphores. `activeSem` (size = `WORKER_COUNT`, default NumCPU) is the real
  concurrency cap that keeps memory flat; `queueSem` (2×) is back-pressure.
  `ProcessStream` runs presets concurrently via errgroup, streaming each `Result`
  through the `onResult` callback. Per-preset failures live in `Result.Err` so one
  bad preset doesn't sink the others; the returned error is reserved for context
  cancellation.

- **`processor/preset.go`** (tag-free) — the canonical preset registry
  (`AllPresets`, `PresetByName`). A `Preset` carries format, dimensions (0 =
  keep original), and encoding knobs. `Preset.Kind` selects output shape:
  `KindImage` (one file) vs `KindFaviconPack` (a multi-file set bundled under a
  `<preset>/` folder in the ZIP). This dual-output plumbing flows through
  `Result.Data` (single) vs `Result.Files` (pack), and into `outFile` in the
  handlers layer.

- **Favicon pack** — `KindFaviconPack` is generated from one center-cropped
  square master into a full drop-in icon set (PNG sizes, `apple-touch-icon`,
  `site.webmanifest`, an HTML snippet, and a hand-built multi-size `favicon.ico`).
  The ICO container is assembled in `processor/ico.go` (tag-free, tested) and the
  vips-side generation in `processor/favicon_vips.go`.

- **`config/config.go`** (tag-free) — all config from env vars, read once at
  startup, logged. Invalid/non-positive numeric values fall back to defaults
  rather than failing startup. `MaxUploadBytes` (Fiber `BodyLimit`) is derived
  from `MAX_FILE_SIZE_MB` plus headroom for multipart boundaries.

Key env vars: `PORT` (3000), `MAX_FILE_SIZE_MB` (50), `WORKER_COUNT` (NumCPU),
`JOB_TTL_MINUTES` (10). See README for the full table.

## Conventions that matter

- **Adding/changing a preset**: edit the `presets` registry in
  `processor/preset.go` in spec order. The frontend selects presets by these exact
  `Name` strings, and `selectPresets` rejects unknown names — keep names stable.
- **The SSE wire schema** is `progressEvent` in `handlers/store.go`; its JSON tags
  (`job`, `preset`, `pct`, `status`, `downloadUrl`) are exactly what the Svelte
  frontend consumes. Changing a tag is a frontend-breaking change.
- **`frontend/dist`** has a committed stub `index.html` so `go build` works without
  running Vite first; the Docker frontend stage overwrites it with the real build.
  Don't delete the stub.
- `runJob` uses `context.Background()`, not the request context — the upload
  handler returns (and cancels its context) the moment the job is queued.
