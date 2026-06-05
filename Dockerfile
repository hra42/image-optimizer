# syntax=docker/dockerfile:1

# ---- Stage 1: build the Svelte frontend ----
FROM node:22-slim AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- Stage 2: build the Go binary (cgo + libvips) ----
FROM golang:1.26-bookworm AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overwrite the stub dist/ with the real Vite build from stage 1.
COPY --from=frontend /app/frontend/dist ./frontend/dist
# The `vips` tag compiles the govips/libvips integration (headers from
# libvips-dev above). Local builds omit it so the toolchain works without libvips.
RUN CGO_ENABLED=1 GOOS=linux go build -tags vips -o /app/image-optimizer .

# ---- Stage 3: minimal runtime ----
FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips42 ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 app
COPY --from=builder /app/image-optimizer /usr/local/bin/image-optimizer
USER app
EXPOSE 3000
ENTRYPOINT ["/usr/local/bin/image-optimizer"]
