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

## The `onnx` build tag — background removal

The `remove_bg` preset (AI background removal, BiRefNet general-lite via ONNX
Runtime) is gated behind a **second, independent build tag: `onnx`**. It composes with `vips` rather
than nesting under it:

- `//go:build onnx` file (`processor/bg_onnx.go`) holds all onnxruntime code and
  does its **own** image decode/encode with the Go stdlib + `golang.org/x/image`
  (NOT govips) — deliberately, so the `onnx` capability stays orthogonal to `vips`
  and there's no 4-way tag matrix. Its `init()` registers the implementation.
- The dispatch seam `processor/bg.go` is **tag-free**: it holds `ErrONNXNotBuilt`,
  the `removeBackgroundFn` hook, and `ConfigureBackground`. When the `onnx` file
  isn't compiled in, the hook is nil and `remove_bg` fails (only that preset) with
  `ErrONNXNotBuilt`. Keep `bg.go` free of any onnxruntime import.
- `pipeline.go` (vips) dispatches `KindBackgroundRemove` to `removeBackground` —
  reached only in a `vips` build, so in pure stub mode `remove_bg` returns
  `ErrVipsNotBuilt` before the onnx seam (acceptable: stub mode processes nothing).
- The onnxruntime binding (`yalue/onnxruntime_go`) `dlopen`s the shared library at
  runtime, so `go build -tags onnx` needs only CGO + a C compiler (no `.so` or
  headers at build time). The `.so` and the model are vendored into the Docker
  image and located via `ONNX_MODEL_PATH` / `ONNXRUNTIME_LIB_PATH` (config).
- **Version pin:** the binding tag and the onnxruntime `.so` must agree on the
  C-API version (`onnxruntime_go v1.31.0` ↔ onnxruntime `1.26.0`). The Dockerfile
  `ARG ONNXRUNTIME_VERSION` and the `config` default `.so` path encode this — bump
  them together.
- **Model is BiRefNet general-lite** (1024² input, ImageNet normalization, sigmoid
  on the output logits). The preprocessing in `bg_onnx.go` is **model-specific** —
  swapping `ONNX_MODEL_PATH` to a different rembg model (u2netp 320²/no-sigmoid,
  isnet 1024²/mean-0.5-std-1.0/no-sigmoid, etc.) needs matching code changes to
  `bgInputSize`, the `bgMean`/`bgStd` constants, and the sigmoid step. The Dockerfile
  fetches the model by MD5; bump that too if you change models.

Local `go build -tags onnx ./...` type-checks `bg_onnx.go` (CGO required); the
production image builds with `-tags "vips onnx"`.

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

Type-check the onnx-tagged background-removal code (CGO required; the binding
vendors its own headers, so no onnxruntime install needed to compile):
```sh
CGO_ENABLED=1 go build -tags onnx ./processor/
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
