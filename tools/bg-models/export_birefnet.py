"""Export BiRefNet_lite (MIT) to a WebGPU-friendly ONNX model.

The stock community exports fail or crawl in onnxruntime-web because of how
torchvision's deform_conv2d is lowered: the usual decomposition samples all
k*k taps at once, materializing a (C * k*k * H * W) tensor -- ~800 MB for the
7x7 ASPP branch at 256x256 -- and emits ops without WebGPU kernels. Here every
DeformableConv2d is swapped for an equivalent that handles one kernel tap at a
time with GridSample (WebGPU-supported) and a 1x1 conv, accumulating the
result, so peak memory stays at one feature map.

The graph also bakes in the final sigmoid, so the browser gets a probability
mask in [0, 1] directly.

Usage: uv run export_birefnet.py [--size 1024] [--fp32]
"""

import argparse
import hashlib
import types
from pathlib import Path

import numpy as np
import onnx
import onnxslim
from onnx import numpy_helper
import torch
import torch.nn.functional as F
from onnxconverter_common import float16
from transformers import AutoModelForImageSegmentation

from common import (
    BIREFNET_REPO, BIREFNET_REV, DIST, EXPORT_VERSION, IMAGENET_MEAN,
    IMAGENET_STD, write_manifest,
)

OPSET = 18

# Ops kept in fp32 inside the fp16 graph: normalization and softmax statistics
# overflow/underflow in half precision, and GridSample's normalized coordinates
# lose sub-pixel accuracy at fp16 on large feature maps.
FP32_OPS = ["LayerNormalization", "ReduceMean", "Pow", "Sqrt", "Softmax", "GridSample", "Sigmoid"]


def deform_conv_forward(self, x):
    """Drop-in for BiRefNet's DeformableConv2d.forward (modulated DCNv2).

    Matches torchvision.ops.deform_conv2d with offset_groups=1, dilation=1:
    tap k=(i,j) samples x at (ho*s - p + i + dy_k, wo*s - p + j + dx_k) with
    bilinear interpolation and zeros outside, scaled by the modulator.
    """
    offset = self.offset_conv(x)
    modulator = 2.0 * torch.sigmoid(self.modulator_conv(x))

    weight = self.regular_conv.weight  # (Cout, Cin, kh, kw)
    kh, kw = weight.shape[-2:]
    sh, sw = self.stride
    ph, pw = (self.padding, self.padding) if isinstance(self.padding, int) else self.padding
    _, _, H, W = x.shape
    Ho, Wo = offset.shape[-2:]

    # Base sampling grid in input pixel coordinates.
    ys = (torch.arange(Ho, dtype=x.dtype, device=x.device) * sh - ph).view(1, Ho, 1)
    xs = (torch.arange(Wo, dtype=x.dtype, device=x.device) * sw - pw).view(1, 1, Wo)

    out = None
    for i in range(kh):
        for j in range(kw):
            k = i * kw + j
            y = ys + i + offset[:, 2 * k]
            xx = xs + j + offset[:, 2 * k + 1]
            # align_corners=True maps -1/1 to pixel centers 0 and W-1, matching
            # deform_conv2d's pixel-index coordinates.
            grid = torch.stack((xx * (2.0 / (W - 1)) - 1.0, y * (2.0 / (H - 1)) - 1.0), dim=-1)
            tap = F.grid_sample(x, grid, mode="bilinear", padding_mode="zeros", align_corners=True)
            tap = tap * modulator[:, k : k + 1]
            contrib = F.conv2d(tap, weight[:, :, i : i + 1, j : j + 1])
            out = contrib if out is None else out + contrib

    if self.regular_conv.bias is not None:
        out = out + self.regular_conv.bias.view(1, -1, 1, 1)
    return out


class Wrapped(torch.nn.Module):
    """Normalized RGB in, sigmoid probability mask out."""

    def __init__(self, model):
        super().__init__()
        self.model = model

    def forward(self, x):
        return torch.sigmoid(self.model(x)[-1])


def dedupe_initializers(m: onnx.ModelProto) -> int:
    """Share weights the tracer duplicated; returns the number of tensors dropped.

    BiRefNet runs its backbone twice (full and half resolution, mul_scl_ipt=cat).
    After slimming, one pass consumes each linear weight through MatMul and the
    other through Gemm with a separately folded *transposed* copy -- ~100 MB of
    duplicates. Byte-identical tensors are merged, and a Gemm whose B is the
    transpose of an existing tensor is repointed at it with transB flipped.
    """
    def digest(a):
        return hashlib.sha256(np.ascontiguousarray(a).tobytes()).hexdigest() + str(a.shape) + str(a.dtype)

    arrays = {i.name: numpy_helper.to_array(i) for i in m.graph.initializer}
    canonical, rename = {}, {}
    for name, a in arrays.items():
        rename[name] = canonical.setdefault(digest(a), name)

    for node in m.graph.node:
        for idx, name in enumerate(node.input):
            if rename.get(name, name) != name:
                node.input[idx] = rename[name]
        if node.op_type != "Gemm" or node.input[1] not in arrays:
            continue
        b = arrays[node.input[1]]
        other = canonical.get(digest(b.T))
        if other is None or other == node.input[1]:
            continue
        node.input[1] = other
        attr = next((a for a in node.attribute if a.name == "transB"), None)
        if attr is None:
            node.attribute.append(onnx.helper.make_attribute("transB", 1))
        else:
            attr.i = 1 - attr.i

    used = {x for n in m.graph.node for x in n.input}
    keep = [i for i in m.graph.initializer if i.name in used]
    dropped = len(m.graph.initializer) - len(keep)
    del m.graph.initializer[:]
    m.graph.initializer.extend(keep)
    return dropped


def load_model():
    model = AutoModelForImageSegmentation.from_pretrained(
        BIREFNET_REPO, revision=BIREFNET_REV, trust_remote_code=True
    )
    model.eval().float()
    patched = 0
    for m in model.modules():
        if type(m).__name__ == "DeformableConv2d":
            m.forward = types.MethodType(deform_conv_forward, m)
            patched += 1
    print(f"patched {patched} DeformableConv2d modules")
    return model


def export(size: int, fp32: bool) -> Path:
    model = Wrapped(load_model())
    dummy = torch.randn(1, 3, size, size)

    variant = f"birefnet-lite-{size}" + ("-fp32" if fp32 else "")
    out_dir = DIST / variant / EXPORT_VERSION
    out_dir.mkdir(parents=True, exist_ok=True)
    raw = out_dir / "raw.onnx"
    final = out_dir / "model.onnx"

    with torch.inference_mode():
        torch.onnx.export(
            model, (dummy,), str(raw), dynamo=False, opset_version=OPSET,
            input_names=["input"], output_names=["mask"], do_constant_folding=True,
        )

    m = onnxslim.slim(onnx.load(str(raw)))
    print(f"deduplicated {dedupe_initializers(m)} initializers")
    if not fp32:
        m = float16.convert_float_to_float16(
            m, keep_io_types=True, op_block_list=FP32_OPS, disable_shape_infer=False
        )
    onnx.save(m, str(final))
    raw.unlink()

    write_manifest(out_dir, {
        "file": final.name,
        "model": variant,
        "upstream": f"{BIREFNET_REPO}@{BIREFNET_REV}",
        "license": "MIT",
        "input": {"name": "input", "size": size, "mean": IMAGENET_MEAN, "std": IMAGENET_STD},
        "output": {"name": "mask", "activation": "sigmoid"},
        "precision": "fp32" if fp32 else "fp16",
    })
    return final


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--size", type=int, default=1024)
    ap.add_argument("--fp32", action="store_true", help="skip fp16 conversion")
    args = ap.parse_args()
    export(args.size, args.fp32)
