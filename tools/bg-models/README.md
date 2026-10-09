# Background-removal models

Builds and verifies the ONNX models the browser uses for the "Remove background"
feature. Inference runs client-side (onnxruntime-web in a Web Worker); these
files are static assets the operator hosts somewhere with CORS, e.g. a
Cloudflare R2 bucket, and points the app at via `BG_MODEL_BASE_URL`.

| Model | Used when | License | File |
|---|---|---|---|
| BiRefNet_lite, 1024², fp16 | Browser has WebGPU with `shader-f16` | MIT | `birefnet-lite-1024/1/model.onnx` (~101 MB) |
| ormbg (IS-Net), 1024², fp16 weights / fp32 compute | Fallback: no WebGPU, or the GPU run fails | Apache-2.0 | `ormbg/1/model.onnx` (~88 MB) |

## Why our own BiRefNet export

The stock BiRefNet ONNX exports fail on onnxruntime-web's WebGPU backend or fall
back to the CPU, because of how `torchvision.ops.deform_conv2d` gets lowered: the
usual decomposition samples every kernel tap at once, materializing a
`C·k²·H·W` tensor (~800 MB for the 7×7 ASPP branch at 256²). `export_birefnet.py`
replaces each deformable conv with an exactly equivalent per-tap
`GridSample` + 1×1 conv accumulation (checked against torchvision in
`test_deform.py`), so peak memory is one feature map. It also:

- bakes the final sigmoid into the graph (the output is a [0, 1] mask),
- merges the duplicated (transposed) backbone weights the tracer folds for
  BiRefNet's second, half-resolution backbone pass (~100 MB saved),
- converts to fp16, keeping LayerNorm, Softmax, GridSample and Sigmoid in fp32.

For ormbg, the published int8 build is ~6× *slower* than fp32 on
onnxruntime-web's WASM backend (~43 s vs ~7 s single-threaded in Chromium, since
`ConvInteger` has no optimized WASM kernel). `export_ormbg.py` therefore stores
fp16 weights behind fp32 casts: half the download, fp32 speed.

`webgpu_check.py` verifies every op has a kernel in the pinned onnxruntime
WebGPU execution provider and that no node exceeds 8 storage-buffer bindings.

## Usage

Requires [uv](https://docs.astral.sh/uv/). CPU-only; no GPU needed.

```sh
cd tools/bg-models
uv run python test_deform.py                          # rewrite == torchvision
uv run python export_birefnet.py                      # -> dist/birefnet-lite-1024/1/
uv run python webgpu_check.py dist/birefnet-lite-1024/1/model.onnx
uv run python verify.py dist/birefnet-lite-1024/1/model.onnx
uv run python export_ormbg.py                         # -> dist/ormbg/1/ (verifies vs fp32)
```

`verify.py` compares the exported model with the unpatched PyTorch model on
sample photos (downloaded from a pinned revision, not committed) and fails if
the mask MAE exceeds 0.01 or IoU drops below 0.98. Reference results for
export version 1:

| Model | Max mask MAE | Min IoU |
|---|---|---|
| birefnet-lite-1024 fp16 | 0.00012 | 0.9996 |
| ormbg fp16-weights vs fp32 | 0.00001 | 1.0000 |

Each output directory gets a `manifest.json` with the file's size and sha256.
**Copy the size and hash into `frontend/src/lib/bgModels.js`**; the browser
refuses any model whose hash doesn't match.

Upstream sources are pinned in `common.py`. Changing the export recipe means
bumping `EXPORT_VERSION` (it's part of the URL path, so caches never serve a
stale model).

## Hosting on Cloudflare R2

Upload the model files with long-lived, immutable caching. The paths are
versioned, so they never change in place:

```sh
for f in birefnet-lite-1024/1/model.onnx ormbg/1/model.onnx; do
  npx wrangler r2 object put "<bucket>/bg/$f" --remote --file "dist/$f" \
    --content-type application/octet-stream \
    --cache-control "public, max-age=31536000, immutable"
done
```

Allow the app's origin to read them (bucket → Settings → CORS policy):

```json
[{ "AllowedOrigins": ["https://your-app.example"], "AllowedMethods": ["GET"], "AllowedHeaders": ["*"], "MaxAgeSeconds": 86400 }]
```

Then expose the bucket on a custom domain and run the app with
`BG_MODEL_BASE_URL=https://models.your-domain.example/bg`. Any static host with
CORS works the same way. Leave the variable unset to hide the feature.

## License note

The BiRefNet weights are MIT. Its training data includes DIS5K, whose terms
restrict commercial use of the *dataset*. The model license doesn't carry that
restriction, but treat it as a residual risk if that matters for your
deployment.
