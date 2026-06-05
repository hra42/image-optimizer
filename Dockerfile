# syntax=docker/dockerfile:1

# ---- Stage 1: build the Svelte frontend ----
FROM node:22-slim AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- Stage 2: build the Go binary (cgo + libvips + onnxruntime) ----
FROM golang:1.26-bookworm AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips-dev pkg-config curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Vendor the onnxruntime shared library and the U²-Netp model used by the
# remove_bg preset. The onnxruntime version MUST match the C-API version the
# yalue/onnxruntime_go binding tracks (1.26.0 for v1.31.0); the binding dlopens
# the .so at runtime, so only the shared object is needed (no headers/apt pkg).
# TARGETARCH is provided by BuildKit; map it to onnxruntime's asset arch naming so
# multi-arch builds (amd64 / arm64) fetch the matching .so — a mismatched arch
# loads but fails dlopen at runtime ("cannot open shared object file").
ARG ONNXRUNTIME_VERSION=1.26.0
ARG TARGETARCH
RUN case "${TARGETARCH}" in \
        amd64) ORT_ARCH=x64 ;; \
        arm64) ORT_ARCH=aarch64 ;; \
        *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; \
    esac \
    && curl -fsSL -o /tmp/ort.tgz \
        "https://github.com/microsoft/onnxruntime/releases/download/v${ONNXRUNTIME_VERSION}/onnxruntime-linux-${ORT_ARCH}-${ONNXRUNTIME_VERSION}.tgz" \
    && mkdir -p /opt/onnxruntime \
    && tar -xzf /tmp/ort.tgz -C /opt/onnxruntime --strip-components=1 \
    && rm /tmp/ort.tgz
# BiRefNet general (lite) — the SwinT-backbone variant of the 2024 BiRefNet
# segmentation model (~224 MB), rembg's practical-on-CPU SOTA general remover.
# Pin to the published asset and verify its MD5 so a corrupted/substituted
# download fails the build. Renamed to a stable path the config default expects.
RUN curl -fsSL -o /tmp/birefnet-general-lite.onnx \
        "https://github.com/danielgatis/rembg/releases/download/v0.0.0/BiRefNet-general-bb_swin_v1_tiny-epoch_232.onnx" \
    && echo "4fab47adc4ff364be1713e97b7e66334  /tmp/birefnet-general-lite.onnx" | md5sum -c -

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overwrite the stub dist/ with the real Vite build from stage 1.
COPY --from=frontend /app/frontend/dist ./frontend/dist
# The `vips` tag compiles the govips/libvips integration (headers from
# libvips-dev above); the `onnx` tag compiles the onnxruntime background-removal
# pipeline (the binding vendors its own C-API headers, so no dev package needed).
# Local builds omit both so the toolchain works without libvips/onnxruntime.
RUN CGO_ENABLED=1 GOOS=linux go build -tags "vips onnx" -o /app/image-optimizer .

# ---- Stage 3: minimal runtime ----
FROM debian:bookworm-slim AS runtime
# libstdc++6 is required by the onnxruntime shared library; libvips42 by the
# image pipeline.
RUN apt-get update && apt-get install -y --no-install-recommends \
        libvips42 libstdc++6 ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 app
COPY --from=builder /app/image-optimizer /usr/local/bin/image-optimizer
# Vendor the onnxruntime shared library and the U²-Netp model at the paths the
# config defaults expect (ONNXRUNTIME_LIB_PATH / ONNX_MODEL_PATH). The binding
# dlopens the versioned .so by name, so copy the whole lib dir to preserve the
# SONAME symlinks.
ARG ONNXRUNTIME_VERSION=1.26.0
COPY --from=builder /opt/onnxruntime/lib/ /usr/local/lib/
COPY --from=builder /tmp/birefnet-general-lite.onnx /usr/local/share/onnx/birefnet-general-lite.onnx
RUN ldconfig
USER app
EXPOSE 3000
# The binary self-probes via -healthcheck (GET /health), so the minimal runtime
# image needs no curl/wget.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/image-optimizer", "-healthcheck"]
ENTRYPOINT ["/usr/local/bin/image-optimizer"]
