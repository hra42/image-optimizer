// Background-removal worker. Runs entirely off the main thread so a multi-second
// inference never freezes the UI. Protocol (all messages carry the request id):
//
//   in:  { id, baseUrl, modelKey, bitmap }   bitmap: ImageBitmap (transferred)
//   out: { id, type: 'progress', phase: 'download'|'load'|'infer', loaded?, total? }
//        { id, type: 'done', blob, model }  blob: image/png with alpha
//        { id, type: 'error', message, fallback? }  fallback: model to retry with
//
// The image never leaves the browser: only the model files are fetched.

// The webgpu build carries both the WebGPU and the WASM execution providers, so
// one runtime serves either model. Its .wasm is emitted by Vite as a hashed
// same-origin asset (resolved via import.meta.url) — no CDN.
import * as ort from 'onnxruntime-web/webgpu';
import { BG_MODELS, MODEL_CACHE, modelUrl } from './bgModels.js';

// Single-threaded WASM: multithreading needs SharedArrayBuffer, i.e. COOP/COEP
// headers that would break cross-origin embeds of the app.
ort.env.wasm.numThreads = 1;

const sessions = new Map(); // modelKey -> Promise<InferenceSession>

self.onmessage = async ({ data }) => {
  const { id, baseUrl, modelKey, bitmap } = data;
  const post = (msg, transfer) => self.postMessage({ id, ...msg }, transfer ?? []);
  const model = BG_MODELS[modelKey];
  try {
    const session = await getSession(modelKey, model, baseUrl, (p) =>
      post({ type: 'progress', ...p }),
    );
    post({ type: 'progress', phase: 'infer' });
    const blob = await cutout(session, model, bitmap);
    post({ type: 'done', blob, model: modelKey });
  } catch (err) {
    sessions.delete(modelKey);
    // A WebGPU failure (adapter lost, OOM, missing feature) is recoverable by
    // the CPU model; anything on the CPU path is final.
    post({
      type: 'error',
      message: err instanceof Error ? err.message : String(err),
      fallback: model.backend === 'webgpu' ? 'ormbg' : undefined,
    });
  } finally {
    bitmap.close?.();
  }
};

function getSession(key, model, baseUrl, onProgress) {
  if (!sessions.has(key)) {
    sessions.set(
      key,
      (async () => {
        const bytes = await loadModel(model, modelUrl(baseUrl, model), onProgress);
        onProgress({ phase: 'load' });
        return ort.InferenceSession.create(bytes, {
          executionProviders: [model.backend],
          graphOptimizationLevel: 'all',
        });
      })(),
    );
  }
  return sessions.get(key);
}

// Returns verified model bytes, from Cache Storage when possible. The hash is
// checked on every load (cache included): it's cheap next to inference and
// guards against a poisoned or truncated cache entry.
async function loadModel(model, url, onProgress) {
  const cache = await caches.open(MODEL_CACHE);
  const cached = await cache.match(url);
  if (cached) {
    const bytes = new Uint8Array(await cached.arrayBuffer());
    if ((await sha256(bytes)) === model.sha256) return bytes;
    await cache.delete(url);
  }

  const res = await fetch(url, { mode: 'cors', credentials: 'omit' });
  if (!res.ok || !res.body) throw new Error(`model download failed (${res.status})`);
  const total = model.bytes;
  const bytes = new Uint8Array(total);
  let loaded = 0;
  const reader = res.body.getReader();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (loaded + value.length > total) throw new Error('model is larger than expected');
    bytes.set(value, loaded);
    loaded += value.length;
    onProgress({ phase: 'download', loaded, total });
  }
  if (loaded !== total || (await sha256(bytes)) !== model.sha256) {
    throw new Error('model failed integrity check');
  }
  await cache.put(url, new Response(bytes, { headers: { 'Content-Type': 'application/octet-stream' } }));
  return bytes;
}

async function sha256(bytes) {
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

// Runs the model on a square resize of the image, then scales the mask back to
// the original resolution and uses it as the alpha channel of the original
// pixels — the subject keeps full resolution; only the mask is upsampled.
async function cutout(session, model, bitmap) {
  const { width: w, height: h } = bitmap;
  const s = model.size;

  const small = new OffscreenCanvas(s, s).getContext('2d', { willReadFrequently: true });
  small.drawImage(bitmap, 0, 0, s, s);
  const rgba = small.getImageData(0, 0, s, s).data;
  const plane = s * s;
  const input = new Float32Array(3 * plane);
  for (let i = 0; i < plane; i++) {
    for (let c = 0; c < 3; c++) {
      input[c * plane + i] = (rgba[i * 4 + c] / 255 - model.mean[c]) / model.std[c];
    }
  }

  const out = await session.run({ [model.input]: new ort.Tensor('float32', input, [1, 3, s, s]) });
  const mask = out[model.output].data; // probabilities in [0, 1], s*s
  out[model.output].dispose?.();

  // Mask → grayscale image, then let the canvas do a high-quality upscale.
  const maskImg = new ImageData(s, s);
  for (let i = 0; i < plane; i++) {
    const v = Math.round(clean(mask[i]) * 255);
    maskImg.data[i * 4] = v;
    maskImg.data[i * 4 + 3] = 255;
  }
  const maskCanvas = new OffscreenCanvas(s, s);
  maskCanvas.getContext('2d').putImageData(maskImg, 0, 0);

  const big = new OffscreenCanvas(w, h).getContext('2d', { willReadFrequently: true });
  big.imageSmoothingQuality = 'high';
  big.drawImage(maskCanvas, 0, 0, w, h);
  const alpha = big.getImageData(0, 0, w, h).data;

  big.clearRect(0, 0, w, h);
  big.drawImage(bitmap, 0, 0);
  const px = big.getImageData(0, 0, w, h);
  for (let i = 3; i < px.data.length; i += 4) px.data[i] = (px.data[i] * alpha[i - 3]) / 255;
  big.putImageData(px, 0, 0);
  return big.canvas.convertToBlob({ type: 'image/png' });
}

// Snap near-certain pixels fully on/off: removes the faint haze soft masks leave
// over the background without touching genuine partial alpha (hair, edges).
function clean(p) {
  if (p < 0.02) return 0;
  if (p > 0.98) return 1;
  return p;
}
