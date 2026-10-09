"""Package ormbg (Apache-2.0, IS-Net) as the WASM fallback model.

Browsers without a usable WebGPU adapter can't run BiRefNet (its Swin
activations exhaust the 4 GB wasm32 heap), so they get ormbg, a plain CNN.

Precision matters a lot on onnxruntime-web's WASM backend. Measured in Chromium
(single thread, 1024^2): the published int8 build (dynamic quantization,
ConvInteger) takes ~43 s, while fp32 takes ~7 s, because ConvInteger has no
optimized WASM kernel. fp32 is a 176 MB download, though. So this script stores the
weights as fp16 with a Cast back to fp32 in front of each one. The download
halves to ~88 MB, and onnxruntime folds the casts when the session loads, so
inference runs at fp32 speed. The masks are checked against the fp32 original.

Usage: uv run export_ormbg.py
"""

import sys

import numpy as np
import onnx
import onnxruntime as ort
from huggingface_hub import hf_hub_download
from onnx import TensorProto, helper, numpy_helper

from common import DIST, EXPORT_VERSION, ORMBG_REPO, ORMBG_REV, write_manifest
from verify import load_samples

MAX_MAE = 0.01
MIN_IOU = 0.98
SIZE = 1024


def preprocess(img) -> np.ndarray:
    a = np.asarray(img.resize((SIZE, SIZE)), dtype=np.float32) / 255.0  # rescale only
    return a.transpose(2, 0, 1)[None]


def fp16_weights(m: onnx.ModelProto) -> onnx.ModelProto:
    """Store float weights as fp16, each behind a Cast to fp32 (compute stays fp32)."""
    inits, casts = [], []
    for init in m.graph.initializer:
        a = numpy_helper.to_array(init)
        if a.dtype != np.float32 or a.size <= 16:
            inits.append(init)
            continue
        half = numpy_helper.from_array(a.astype(np.float16), init.name + "_fp16")
        inits.append(half)
        casts.append(helper.make_node("Cast", [half.name], [init.name], to=TensorProto.FLOAT))
    del m.graph.initializer[:]
    m.graph.initializer.extend(inits)
    nodes = list(m.graph.node)
    del m.graph.node[:]
    m.graph.node.extend(casts + nodes)
    onnx.checker.check_model(m)
    return m


def main() -> int:
    fp32 = hf_hub_download(ORMBG_REPO, "onnx/model.onnx", revision=ORMBG_REV)
    out_dir = DIST / "ormbg" / EXPORT_VERSION
    out_dir.mkdir(parents=True, exist_ok=True)
    final = out_dir / "model.onnx"
    onnx.save(fp16_weights(onnx.load(fp32)), str(final))

    q = ort.InferenceSession(str(final), providers=["CPUExecutionProvider"])
    ref = ort.InferenceSession(fp32, providers=["CPUExecutionProvider"])

    failed = 0
    for name, img in load_samples():
        x = preprocess(img)
        got = q.run(None, {"pixel_values": x})[0]
        want = ref.run(None, {"pixel_values": x})[0]
        mae = float(np.abs(got - want).mean())
        a, b = got > 0.5, want > 0.5
        iou = float((a & b).sum() / max((a | b).sum(), 1))
        ok = mae <= MAX_MAE and iou >= MIN_IOU
        failed += not ok
        print(f"{'ok  ' if ok else 'FAIL'} {name:40s} fg={a.mean():.3f} mae={mae:.5f} iou={iou:.4f}")
    if failed:
        final.unlink()
        return 1

    write_manifest(out_dir, {
        "file": "model.onnx",
        "model": "ormbg",
        "upstream": f"{ORMBG_REPO}@{ORMBG_REV} (onnx/model.onnx)",
        "license": "Apache-2.0",
        "input": {"name": "pixel_values", "size": SIZE, "mean": [0, 0, 0], "std": [1, 1, 1]},
        "output": {"name": "alphas", "activation": "sigmoid"},
        "precision": "fp16 weights, fp32 compute",
    })
    return 0


if __name__ == "__main__":
    sys.exit(main())
