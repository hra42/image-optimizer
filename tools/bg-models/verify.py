"""Verify an exported BiRefNet ONNX model against the PyTorch reference.

The reference is the *unpatched* upstream model (torchvision's deform_conv2d),
so this checks the deform-conv rewrite, graph slimming, weight dedupe and fp16
conversion end to end. Masks are compared on sample photos pulled from a pinned
Hugging Face revision (not committed: their licenses are not ours to
redistribute) plus a synthetic logo.

Usage: uv run verify.py dist/birefnet-lite-1024/1/model.onnx
Fails (non-zero exit) when any image exceeds the MAE / IoU thresholds.
"""

import json
import sys
import time
from pathlib import Path

import numpy as np
import onnxruntime as ort
import torch
from huggingface_hub import hf_hub_download
from PIL import Image, ImageDraw
from transformers import AutoModelForImageSegmentation

from common import BIREFNET_REPO, BIREFNET_REV

MAX_MAE = 0.01
MIN_IOU = 0.98

SAMPLES_REPO = "schirrmacher/ormbg"
SAMPLES_REV = "6253b318240ef7a8670017b88d242f9f87f5abeb"
SAMPLES = [
    "examples/loss/orginal.jpg",
    "dataset/validation/im/p_00a7a27c.png",
    "examples/image/image01.png",
]


def synthetic_logo() -> Image.Image:
    img = Image.new("RGB", (800, 600), (235, 240, 245))
    d = ImageDraw.Draw(img)
    d.ellipse((220, 120, 580, 480), fill=(220, 60, 40))
    d.rectangle((340, 240, 460, 360), fill=(255, 255, 255))
    return img


def load_samples():
    for name in SAMPLES:
        path = hf_hub_download(SAMPLES_REPO, name, revision=SAMPLES_REV)
        yield name, Image.open(path).convert("RGB")
    yield "synthetic-logo", synthetic_logo()


def preprocess(img: Image.Image, size: int, mean, std) -> np.ndarray:
    a = np.asarray(img.resize((size, size), Image.BILINEAR), dtype=np.float32) / 255.0
    a = (a - np.array(mean, dtype=np.float32)) / np.array(std, dtype=np.float32)
    return a.transpose(2, 0, 1)[None]


def main(model_path: str) -> int:
    manifest = json.loads((Path(model_path).parent / "manifest.json").read_text())
    size, mean, std = (manifest["input"][k] for k in ("size", "mean", "std"))

    ref = AutoModelForImageSegmentation.from_pretrained(
        BIREFNET_REPO, revision=BIREFNET_REV, trust_remote_code=True
    ).eval().float()
    sess = ort.InferenceSession(model_path, providers=["CPUExecutionProvider"])

    failed = 0
    for name, img in load_samples():
        x = preprocess(img, size, mean, std)
        with torch.inference_mode():
            want = torch.sigmoid(ref(torch.from_numpy(x))[-1]).numpy()
        t = time.perf_counter()
        got = sess.run(None, {"input": x})[0].astype(np.float32)
        dt = time.perf_counter() - t

        mae = float(np.abs(got - want).mean())
        a, b = got > 0.5, want > 0.5
        iou = float((a & b).sum() / max((a | b).sum(), 1))
        ok = mae <= MAX_MAE and iou >= MIN_IOU
        failed += not ok
        print(f"{'ok  ' if ok else 'FAIL'} {name:40s} fg={a.mean():.3f} mae={mae:.6f} iou={iou:.4f} ort_cpu={dt:.1f}s")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
