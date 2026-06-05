//go:build onnx

// Package-local ONNX background removal. Compiled only with the `onnx` build tag
// (the onnxruntime shared library is vendored into the Docker image, not present
// locally). This file is deliberately self-contained: it does its own image I/O
// with the Go standard library + golang.org/x/image instead of going through
// govips, so the `onnx` capability composes with the `vips` tag without a 4-way
// build matrix.
//
// The model is BiRefNet (general lite) — the SwinT-backbone variant of the 2024
// BiRefNet dichotomous-segmentation model, the current practical-on-CPU SOTA for
// general background removal. Pre/post-processing mirrors rembg's BiRefNet session
// exactly so the masks match:
//   - resize to 1024×1024, scale by 1/max(channel-max, 1e-6), ImageNet-normalize
//   - run inference → a 1×1×1024×1024 logit map
//   - sigmoid → min-max normalize → resize back to the source size, use as alpha
//
// The sigmoid is the one postprocessing step BiRefNet needs that the old U²-Netp
// path did not (U²-Net's output was already a saliency map in [0,1]).
//
// References: github.com/danielgatis/rembg (sessions/base.py,
// sessions/birefnet_general.py); github.com/ZhengPeng7/BiRefNet (CAAI AIR'24).

package processor

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // register JPEG decoder for image.Decode
	"image/png"
	"math"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
	xdraw "golang.org/x/image/draw"
)

// init registers this ONNX implementation as the background-removal backend. The
// tag-free dispatch seam (processor/bg.go) calls it for KindBackgroundRemove
// presets; in builds without this file removeBackgroundFn stays nil and the seam
// returns ErrONNXNotBuilt.
func init() {
	removeBackgroundFn = removeBackgroundONNX
}

const (
	// bgInputSize is the fixed square input the model expects. BiRefNet (general
	// lite) runs at 1024×1024 — the higher resolution is what resolves hair and
	// fine edges far better than the old U²-Netp 320².
	bgInputSize = 1024
	// bgInferenceConcurrency caps how many ONNX passes run at once. ORT already
	// uses all cores per inference, so allowing many in parallel would only
	// thrash; at 1024² each BiRefNet pass also holds large activation buffers, so
	// we serialize (1) to keep peak memory bounded. Independent of the libvips
	// activeSem.
	bgInferenceConcurrency = 1
)

// ImageNet normalization constants (per channel, RGB order), matching rembg's
// BiRefNet session.
var (
	bgMean = [3]float32{0.485, 0.456, 0.406}
	bgStd  = [3]float32{0.229, 0.224, 0.225}
)

// bgSession lazily holds the singleton ONNX session and the names discovered
// from the model. Initialization (onnxruntime env + session) happens once on
// first use, guarded by bgOnce; bgInitErr captures a failure so every subsequent
// call reports it rather than retrying a broken setup.
var (
	bgOnce     sync.Once
	bgSession  *ort.DynamicAdvancedSession
	bgInputN   string
	bgOutputN  string
	bgInitErr  error
	bgInferSem = make(chan struct{}, bgInferenceConcurrency)
)

// bgInit performs the one-time onnxruntime initialization: point the loader at
// the vendored shared library, initialize the environment, inspect the model for
// its input/output tensor names, and create a reusable session. Paths come from
// ConfigureBackground (set at startup from config). Safe to call via bgOnce only.
func bgInit() {
	if bgLibPath != "" {
		ort.SetSharedLibraryPath(bgLibPath)
	}
	if err := ort.InitializeEnvironment(); err != nil {
		bgInitErr = fmt.Errorf("onnx init environment: %w", err)
		return
	}

	// Discover the actual input/output node names from the model rather than
	// hardcoding them — rembg exports commonly name the input "input.1", but
	// reading them keeps us correct across re-exports.
	inputs, outputs, err := ort.GetInputOutputInfoWithOptions(bgModelPath, nil)
	if err != nil {
		bgInitErr = fmt.Errorf("onnx inspect model %q: %w", bgModelPath, err)
		return
	}
	if len(inputs) == 0 || len(outputs) == 0 {
		bgInitErr = fmt.Errorf("onnx model %q has %d inputs / %d outputs; need at least one of each", bgModelPath, len(inputs), len(outputs))
		return
	}
	bgInputN = inputs[0].Name
	// BiRefNet has a single output; use the first regardless.
	bgOutputN = outputs[0].Name

	session, err := ort.NewDynamicAdvancedSession(bgModelPath, []string{bgInputN}, []string{bgOutputN}, nil)
	if err != nil {
		bgInitErr = fmt.Errorf("onnx create session: %w", err)
		return
	}
	bgSession = session
}

// removeBackgroundONNX runs the full background-removal pipeline for one image:
// decode → preprocess → infer → mask → composite RGBA → encode PNG. Any failure
// is returned inside the Result so a single bad input doesn't sink other presets.
func removeBackgroundONNX(buf []byte, p Preset) Result {
	res := Result{Preset: p}

	bgOnce.Do(bgInit)
	if bgInitErr != nil {
		res.Err = bgInitErr
		return res
	}

	// Decode the source. The stdlib handles JPEG/PNG (and GIF); WebP/AVIF/HEIC are
	// not supported on this path — those inputs fail here, only for this preset.
	src, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		res.Err = fmt.Errorf("decode %q: %w", p.Name, err)
		return res
	}
	b := src.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		res.Err = fmt.Errorf("decode %q: empty image", p.Name)
		return res
	}

	// Work in RGBA so alpha compositing and resampling are straightforward.
	rgba := image.NewRGBA(image.Rect(0, 0, srcW, srcH))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)

	input := preprocess(rgba)

	mask, err := runInference(input)
	if err != nil {
		res.Err = fmt.Errorf("infer %q: %w", p.Name, err)
		return res
	}

	// Build the full-resolution alpha by resizing the 1024×1024 mask back up, then
	// apply it to the source pixels.
	alpha := resizeMask(mask, srcW, srcH)
	applyAlpha(rgba, alpha)

	var out bytes.Buffer
	enc := png.Encoder{CompressionLevel: pngCompressionLevel(p.Compression)}
	if err := enc.Encode(&out, rgba); err != nil {
		res.Err = fmt.Errorf("encode %q: %w", p.Name, err)
		return res
	}

	res.Data = out.Bytes()
	res.Width = srcW
	res.Height = srcH
	return res
}

// preprocess turns the source image into the model's input tensor data: resize to
// 1024×1024, scale to ~[0,1] by dividing by the per-image channel max (1e-6 floor,
// matching rembg), ImageNet-normalize, and lay out as NCHW float32.
func preprocess(src *image.RGBA) []float32 {
	small := image.NewRGBA(image.Rect(0, 0, bgInputSize, bgInputSize))
	xdraw.CatmullRom.Scale(small, small.Bounds(), src, src.Bounds(), xdraw.Over, nil)

	n := bgInputSize * bgInputSize
	// NCHW: three contiguous planes (R, G, B), each n elements.
	data := make([]float32, 3*n)

	// rembg divides the whole array by its max value (not a fixed 255). For 8-bit
	// input the max is almost always 255, but follow the reference exactly: find
	// the max sample first, with a 1e-6 floor to avoid div-by-zero.
	var maxv float32 = 1e-6
	pix := small.Pix // RGBA, 4 bytes per pixel, row-major
	for i := 0; i < n; i++ {
		r := float32(pix[i*4+0])
		g := float32(pix[i*4+1])
		b := float32(pix[i*4+2])
		if r > maxv {
			maxv = r
		}
		if g > maxv {
			maxv = g
		}
		if b > maxv {
			maxv = b
		}
	}

	for i := 0; i < n; i++ {
		r := float32(pix[i*4+0]) / maxv
		g := float32(pix[i*4+1]) / maxv
		b := float32(pix[i*4+2]) / maxv
		data[0*n+i] = (r - bgMean[0]) / bgStd[0]
		data[1*n+i] = (g - bgMean[1]) / bgStd[1]
		data[2*n+i] = (b - bgMean[2]) / bgStd[2]
	}
	return data
}

// runInference runs one ONNX pass over the preprocessed NCHW input and returns
// the 1024×1024 saliency map (the first output's first channel), with the BiRefNet
// sigmoid applied so values land in [0,1]. Access to the shared session is
// serialized through bgInferSem.
func runInference(input []float32) ([]float32, error) {
	inputShape := ort.NewShape(1, 3, bgInputSize, bgInputSize)
	inputTensor, err := ort.NewTensor(inputShape, input)
	if err != nil {
		return nil, fmt.Errorf("create input tensor: %w", err)
	}
	defer inputTensor.Destroy()

	outputShape := ort.NewShape(1, 1, bgInputSize, bgInputSize)
	outputTensor, err := ort.NewEmptyTensor[float32](outputShape)
	if err != nil {
		return nil, fmt.Errorf("create output tensor: %w", err)
	}
	defer outputTensor.Destroy()

	bgInferSem <- struct{}{}
	err = bgSession.Run([]ort.Value{inputTensor}, []ort.Value{outputTensor})
	<-bgInferSem
	if err != nil {
		return nil, fmt.Errorf("session run: %w", err)
	}

	// Copy out (the tensor's backing slice is reused/destroyed) and apply the
	// BiRefNet sigmoid: the model emits raw logits, so squash to [0,1] here before
	// the downstream min-max normalize, matching rembg's predict().
	raw := outputTensor.GetData()
	out := make([]float32, len(raw))
	for i, v := range raw {
		out[i] = 1.0 / (1.0 + float32(math.Exp(float64(-v))))
	}
	return out, nil
}

// resizeMask min-max normalizes the 1024×1024 saliency map to [0,255] and
// resizes it to the target dimensions, returning an 8-bit single-channel alpha
// plane (row-major, w*h bytes). Matches rembg: normalize → uint8 → resize.
func resizeMask(raw []float32, w, h int) []byte {
	// Min-max normalize so the most-foreground pixel maps to 255.
	var mi, ma float32 = raw[0], raw[0]
	for _, v := range raw {
		if v < mi {
			mi = v
		}
		if v > ma {
			ma = v
		}
	}
	rng := ma - mi
	if rng <= 0 {
		rng = 1e-6
	}

	// Pack the normalized map into a grayscale image so we can resample it with
	// the same high-quality filter used on the way in.
	maskSmall := image.NewGray(image.Rect(0, 0, bgInputSize, bgInputSize))
	for i, v := range raw {
		norm := (v - mi) / rng
		if norm < 0 {
			norm = 0
		} else if norm > 1 {
			norm = 1
		}
		maskSmall.Pix[i] = uint8(norm*255 + 0.5)
	}

	maskFull := image.NewGray(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(maskFull, maskFull.Bounds(), maskSmall, maskSmall.Bounds(), xdraw.Over, nil)
	return maskFull.Pix
}

// applyAlpha sets each pixel's alpha to the mask value and premultiplies the RGB
// by that alpha, so the encoded PNG's transparent regions are correct (Go's RGBA
// is alpha-premultiplied).
func applyAlpha(img *image.RGBA, alpha []byte) {
	for i := 0; i < len(alpha); i++ {
		a := uint32(alpha[i])
		base := i * 4
		// Premultiply existing (opaque) RGB by the new alpha.
		img.Pix[base+0] = uint8(uint32(img.Pix[base+0]) * a / 255)
		img.Pix[base+1] = uint8(uint32(img.Pix[base+1]) * a / 255)
		img.Pix[base+2] = uint8(uint32(img.Pix[base+2]) * a / 255)
		img.Pix[base+3] = alpha[i]
	}
}

// pngCompressionLevel maps the preset's 0–9 zlib compression knob onto the
// stdlib png encoder's discrete levels. The default (and 0, which the registry
// uses to mean "encoder default") map to DefaultCompression.
func pngCompressionLevel(c int) png.CompressionLevel {
	switch {
	case c <= 0:
		return png.DefaultCompression
	case c >= 8:
		return png.BestCompression
	case c <= 2:
		return png.BestSpeed
	default:
		return png.DefaultCompression
	}
}
