package processor

// This file is tag-free: the ICO container format is assembled from already-
// encoded PNG bytes with pure stdlib, so it builds and unit-tests locally
// without libvips. The PNGs themselves are produced by the vips pipeline.

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// icoImage is one PNG to embed in an .ico, with its square pixel dimension.
type icoImage struct {
	size int    // width == height, in pixels (e.g. 16, 32, 48)
	png  []byte // PNG-encoded bytes
}

// buildICO assembles a multi-image ICO from the given PNGs. The ICO format is a
// 6-byte header, then one 16-byte directory entry per image, then the image
// payloads. Modern browsers accept PNG payloads inside ICO, so each entry simply
// embeds the PNG verbatim. Sizes of 256 are encoded as 0 per the spec (the byte
// field maxes at 255); we only use 16/32/48 so this is just spec-correctness.
func buildICO(images []icoImage) ([]byte, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("buildICO: no images")
	}

	var buf bytes.Buffer

	// ICONDIR header: reserved(0), type(1 = icon), count.
	_ = binary.Write(&buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(images)))

	// Image payloads start after the header (6 bytes) and all directory
	// entries (16 bytes each).
	offset := 6 + 16*len(images)

	// ICONDIRENTRY for each image.
	for _, img := range images {
		dim := img.size
		if dim >= 256 {
			dim = 0 // 0 means 256 in the ICO spec
		}
		buf.WriteByte(byte(dim))                                          // width
		buf.WriteByte(byte(dim))                                          // height
		buf.WriteByte(0)                                                  // color palette count (0 = no palette)
		buf.WriteByte(0)                                                  // reserved
		_ = binary.Write(&buf, binary.LittleEndian, uint16(1))            // color planes
		_ = binary.Write(&buf, binary.LittleEndian, uint16(32))           // bits per pixel
		_ = binary.Write(&buf, binary.LittleEndian, uint32(len(img.png))) // payload size
		_ = binary.Write(&buf, binary.LittleEndian, uint32(offset))       // payload offset
		offset += len(img.png)
	}

	// Image payloads, in the same order as the directory entries.
	for _, img := range images {
		buf.Write(img.png)
	}

	return buf.Bytes(), nil
}
