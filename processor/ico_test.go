package processor

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// fakePNG is a stand-in payload; buildICO embeds bytes verbatim and never parses
// them, so the ICO container structure can be validated without real PNGs.
func fakePNG(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func TestBuildICOStructure(t *testing.T) {
	imgs := []icoImage{
		{size: 16, png: fakePNG(10)},
		{size: 32, png: fakePNG(20)},
		{size: 48, png: fakePNG(30)},
	}
	ico, err := buildICO(imgs)
	if err != nil {
		t.Fatalf("buildICO: %v", err)
	}

	// Header: reserved(0), type(1), count.
	if got := binary.LittleEndian.Uint16(ico[0:2]); got != 0 {
		t.Errorf("reserved = %d, want 0", got)
	}
	if got := binary.LittleEndian.Uint16(ico[2:4]); got != 1 {
		t.Errorf("type = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint16(ico[4:6]); got != uint16(len(imgs)) {
		t.Errorf("count = %d, want %d", got, len(imgs))
	}

	// Each directory entry's offset/size must point at the right payload.
	for i, img := range imgs {
		entry := ico[6+16*i : 6+16*i+16]
		if entry[0] != byte(img.size) || entry[1] != byte(img.size) {
			t.Errorf("entry %d dims = %dx%d, want %dx%d", i, entry[0], entry[1], img.size, img.size)
		}
		size := binary.LittleEndian.Uint32(entry[8:12])
		offset := binary.LittleEndian.Uint32(entry[12:16])
		if int(size) != len(img.png) {
			t.Errorf("entry %d size = %d, want %d", i, size, len(img.png))
		}
		got := ico[offset : int(offset)+int(size)]
		if !bytes.Equal(got, img.png) {
			t.Errorf("entry %d payload mismatch", i)
		}
	}
}

func TestBuildICOEmpty(t *testing.T) {
	if _, err := buildICO(nil); err == nil {
		t.Error("buildICO(nil) returned nil error, want failure")
	}
}
