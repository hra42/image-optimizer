"""Checks the per-tap deform conv rewrite against torchvision's reference."""

import types

import torch
from torch import nn
from torchvision.ops import deform_conv2d

from export_birefnet import deform_conv_forward


class Ref(nn.Module):
    def __init__(self, cin, cout, k, pad):
        super().__init__()
        self.stride, self.padding = (1, 1), pad
        self.offset_conv = nn.Conv2d(cin, 2 * k * k, k, padding=pad)
        self.modulator_conv = nn.Conv2d(cin, k * k, k, padding=pad)
        self.regular_conv = nn.Conv2d(cin, cout, k, padding=pad, bias=False)

    def forward(self, x):
        offset = self.offset_conv(x)
        mod = 2.0 * torch.sigmoid(self.modulator_conv(x))
        return deform_conv2d(x, offset, self.regular_conv.weight, self.regular_conv.bias,
                             padding=self.padding, mask=mod, stride=self.stride)


def test_matches_torchvision():
    torch.manual_seed(0)
    for k in (1, 3, 7):
        m = Ref(8, 16, k, k // 2)
        with torch.no_grad():  # large offsets so sampling crosses borders
            m.offset_conv.weight.normal_(0, 2.0)
        x = torch.randn(2, 8, 23, 31)
        want = m(x)
        m.forward = types.MethodType(deform_conv_forward, m)
        got = m(x)
        err = (got - want).abs().max().item()
        assert err < 1e-4, f"k={k}: max abs err {err}"


if __name__ == "__main__":
    test_matches_torchvision()
    print("ok")
