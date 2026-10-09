"""Static WebGPU compatibility check for an ONNX graph.

onnxruntime-web silently falls back to the CPU (WASM) for any node the WebGPU
execution provider has no kernel for, which turns a ~1 s inference into tens of
seconds of GPU<->CPU ping-pong. It also fails at runtime when one shader needs
more storage-buffer bindings than the adapter allows (8 is the WebGPU default
limit). This script flags both before a model ships:

  * ops not registered by the WebGPU EP at the pinned onnxruntime tag (parsed
    from webgpu_execution_provider.cc, cached under .cache/), and
  * nodes whose inputs + outputs exceed MAX_BINDINGS.

Usage: uv run webgpu_check.py path/to/model.onnx
Exit status is non-zero when problems are found.
"""

import re
import sys
import urllib.request
from collections import Counter
from pathlib import Path

import onnx

ORT_TAG = "v1.30.0"  # keep in sync with onnxruntime-web in frontend/package.json
MAX_BINDINGS = 8
SRC_URL = (
    "https://raw.githubusercontent.com/microsoft/onnxruntime/"
    f"{ORT_TAG}/onnxruntime/core/providers/webgpu/webgpu_execution_provider.cc"
)
CACHE = Path(__file__).parent / ".cache" / f"webgpu_ep_{ORT_TAG}.cc"

# Registered through helper functions rather than the kernel macros.
BINARY_ELEMENTWISE = {
    "Add", "Sub", "Mul", "Div", "Max", "Min", "Equal", "Greater", "Less",
    "GreaterOrEqual", "LessOrEqual", "Pow", "PRelu", "And",
}
# Shape-only ops ORT resolves on the CPU by design (cheap, no tensor data).
CPU_BY_DESIGN = {"Shape", "Constant", "ConstantOfShape", "Range", "Size"}


def webgpu_ops() -> set[str]:
    if not CACHE.exists():
        CACHE.parent.mkdir(parents=True, exist_ok=True)
        with urllib.request.urlopen(SRC_URL) as r:
            CACHE.write_bytes(r.read())
    src = CACHE.read_text()
    ops = set()
    for args in re.findall(r"KERNEL_CREATE_INFO[A-Z_]*\(([^)]*)\)", src):
        ops.add(args.split(",")[-1].strip())
    for args in re.findall(r"kWebGpuExecutionProvider, kOnnxDomain, ([^)]*)\)", src):
        ops.add(args.split(",")[-1].strip())
    ops.update(re.findall(r"Create(\w+?)(?:Versioned)?KernelInfo\(", src))
    return ops | BINARY_ELEMENTWISE


def check(path: str) -> list[str]:
    model = onnx.load(path, load_external_data=False)
    supported = webgpu_ops()
    problems = []

    unsupported = Counter(
        n.op_type for n in model.graph.node
        if n.domain in ("", "ai.onnx")
        and n.op_type not in supported
        and n.op_type not in CPU_BY_DESIGN
    )
    for op, count in sorted(unsupported.items()):
        problems.append(f"no WebGPU kernel: {op} x{count}")

    for n in model.graph.node:
        bindings = sum(1 for i in n.input if i) + sum(1 for o in n.output if o)
        if bindings > MAX_BINDINGS:
            problems.append(f"{n.op_type} {n.name!r}: {bindings} bindings > {MAX_BINDINGS}")
    return problems


if __name__ == "__main__":
    problems = check(sys.argv[1])
    for p in problems:
        print(p)
    print(f"{len(problems)} problem(s)")
    sys.exit(1 if problems else 0)
