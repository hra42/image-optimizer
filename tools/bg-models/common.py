"""Shared constants and helpers for the background-removal model tooling."""

import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).parent
DIST = ROOT / "dist"

# Upstream sources, pinned to exact revisions so every export is reproducible.
BIREFNET_REPO = "ZhengPeng7/BiRefNet_lite"
BIREFNET_REV = "aa62cd87eafb9cc43056d08ef3615a14628b831d"
ORMBG_REPO = "onnx-community/ormbg-ONNX"
ORMBG_REV = "034e2d884afbab897e10e78fc5bb566b29533fd6"

# Bump when the export recipe changes; it is part of the published URL path, so
# browsers and the CDN can cache every file forever.
EXPORT_VERSION = "1"

IMAGENET_MEAN = [0.485, 0.456, 0.406]
IMAGENET_STD = [0.229, 0.224, 0.225]


def sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def write_manifest(out_dir: Path, entry: dict) -> None:
    """Record a produced model file (name, size, sha256 + caller metadata)."""
    model = out_dir / entry["file"]
    entry = {**entry, "size": model.stat().st_size, "sha256": sha256(model)}
    (out_dir / "manifest.json").write_text(json.dumps(entry, indent=2) + "\n")
    print(json.dumps(entry, indent=2))
