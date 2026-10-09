// Background-removal models, pinned by content hash. Files are fetched from the
// operator-configured base URL (BG_MODEL_BASE_URL, served at /config) and
// verified against these hashes before use, so a tampered or stale bucket can't
// swap the model. Paths are versioned and immutable: re-export → new path.
// Produced by tools/bg-models (see its README); keep in sync with the manifests.

const IMAGENET_MEAN = [0.485, 0.456, 0.406];
const IMAGENET_STD = [0.229, 0.224, 0.225];

export const BG_MODELS = {
  // BiRefNet_lite (MIT), exported for the WebGPU EP. Best edges/hair; needs a
  // GPU — its transformer activations don't fit the 4 GB wasm32 heap.
  birefnet: {
    label: 'BiRefNet',
    path: 'birefnet-lite-1024/1/model.onnx',
    sha256: '115ae881adf3ddd4c04ceb9709c391d60413602125defcfb9c0734a725bec7cb',
    bytes: 101455779,
    size: 1024,
    mean: IMAGENET_MEAN,
    std: IMAGENET_STD,
    input: 'input',
    output: 'mask',
    backend: 'webgpu',
  },
  // ormbg (Apache-2.0, IS-Net): fp16-stored weights, fp32 compute. CPU fallback
  // for browsers without WebGPU (int8 would be ~6x slower on WASM).
  ormbg: {
    label: 'ormbg',
    path: 'ormbg/1/model.onnx',
    sha256: '3e22154ae7db795ef140788e5520b1ced56acabc3f7c8bdd2bb2c3d5c2edd6a3',
    bytes: 88120364,
    size: 1024,
    mean: [0, 0, 0],
    std: [1, 1, 1],
    input: 'pixel_values',
    output: 'alphas',
    backend: 'wasm',
  },
};

// Cache Storage bucket for verified model bytes. Bump the suffix to evict.
export const MODEL_CACHE = 'bg-models-v1';

export function modelUrl(baseUrl, model) {
  return `${baseUrl}/${model.path}`;
}
