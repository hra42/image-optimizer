# syntax=docker/dockerfile:1

# ---- Stage 1: build the Svelte frontend ----
FROM node:22-slim AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- Stage 2: build the Go binary (cgo + libvips) ----
# Trixie (not bookworm) for its newer libheif: bookworm's libheif 1.15 rejects
# iPhone HEICs during decode ("Metadata not correctly assigned to image");
# trixie's 1.19 decodes them fine. Builder and runtime bases must match.
FROM golang:1.26-trixie AS builder
# libheif-dev provides the HEIF/HEIC loader headers libvips' heifload links
# against — libvips-dev only recommends it, so install it explicitly or HEIC
# decoding is silently absent from the build.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips-dev libheif-dev pkg-config curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# VERSION is the release string shown in the UI footer (via /version). It is
# passed in at build time because .git is dockerignored, so the build can't run
# `git describe` itself — compute it on the host and forward it:
#   docker build --build-arg VERSION="$(git describe --tags --always)" .
# Falls back to "dev" when not supplied.
ARG VERSION=dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overwrite the stub dist/ with the real Vite build from stage 1.
COPY --from=frontend /app/frontend/dist ./frontend/dist
# The `vips` tag compiles the govips/libvips integration (headers from
# libvips-dev above). Local builds omit it so the toolchain works without libvips.
# -X main.version stamps the build version into the binary.
RUN CGO_ENABLED=1 GOOS=linux go build -tags "vips" \
        -ldflags "-X main.version=${VERSION}" \
        -o /app/image-optimizer .

# ---- Stage 3: minimal runtime ----
FROM debian:trixie-slim AS runtime
# libvips42t64 is required by the image pipeline (the package was renamed from
# libvips42 in trixie's time_t-64 transition). libheif1 + libde265-0 add HEIC/HEIF
# decoding: libvips does not depend on libheif, and libheif itself needs an HEVC
# decoder (libde265) or it loads the container but fails the bitstream with
# "Unsupported codec". Without these, HEIC uploads decode to nothing → empty ZIP.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips42t64 libheif1 libde265-0 ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 app
COPY --from=builder /app/image-optimizer /usr/local/bin/image-optimizer
USER app
EXPOSE 3000
# The binary self-probes via -healthcheck (GET /health), so the minimal runtime
# image needs no curl/wget.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/image-optimizer", "-healthcheck"]
ENTRYPOINT ["/usr/local/bin/image-optimizer"]
