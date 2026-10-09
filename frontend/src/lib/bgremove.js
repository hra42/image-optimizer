// Main-thread client for the background-removal worker. Picks the model for
// this device, reports whether it still needs downloading, and runs a cutout.

import { BG_MODELS, MODEL_CACHE, modelUrl } from './bgModels.js';

let worker = null;
let nextId = 0;
const pending = new Map(); // id -> { onProgress, resolve, reject }

function getWorker() {
  if (!worker) {
    worker = new Worker(new URL('./bgremove.worker.js', import.meta.url), { type: 'module' });
    worker.onmessage = ({ data }) => {
      const req = pending.get(data.id);
      if (!req) return;
      if (data.type === 'progress') {
        req.onProgress?.(data);
        return;
      }
      pending.delete(data.id);
      if (data.type === 'done') req.resolve(data);
      else req.reject(Object.assign(new Error(data.message), { fallback: data.fallback }));
    };
  }
  return worker;
}

// BiRefNet when the browser exposes a WebGPU adapter with fp16 shaders (the
// model computes in half precision), otherwise the CPU fallback.
let picked = null;
export function pickModel() {
  picked ??= (async () => {
    try {
      const adapter = await navigator.gpu?.requestAdapter();
      if (adapter?.features.has('shader-f16')) return 'birefnet';
    } catch {
      // fall through to the CPU model
    }
    return 'ormbg';
  })();
  return picked;
}

// True when the model's bytes are already in Cache Storage (no download needed).
export async function isModelCached(baseUrl, modelKey) {
  try {
    const cache = await caches.open(MODEL_CACHE);
    return !!(await cache.match(modelUrl(baseUrl, BG_MODELS[modelKey])));
  } catch {
    return false;
  }
}

function run(file, baseUrl, modelKey, onProgress) {
  return createImageBitmap(file, { imageOrientation: 'from-image' }).then(
    (bitmap) =>
      new Promise((resolve, reject) => {
        const id = ++nextId;
        pending.set(id, { onProgress, resolve, reject });
        getWorker().postMessage({ id, baseUrl, modelKey, bitmap }, [bitmap]);
      }),
  );
}

// Removes the background from `file`. Resolves { blob, model } where blob is a
// PNG with alpha at the original resolution (EXIF orientation baked in). If the
// GPU model fails at runtime, retries once on the CPU model; onProgress then
// sees a second download phase for it.
export async function removeBackground(file, baseUrl, onProgress) {
  const modelKey = await pickModel();
  try {
    return await run(file, baseUrl, modelKey, onProgress);
  } catch (err) {
    if (!err.fallback) throw err;
    console.warn(`background removal: ${modelKey} failed (${err.message}); retrying with ${err.fallback}`);
    picked = Promise.resolve(err.fallback); // don't retry the GPU path this session
    return run(file, baseUrl, err.fallback, onProgress);
  }
}
