package processor

import "testing"

func TestAILabelPlacement(t *testing.T) {
	tests := []struct {
		name     string
		position string
		wantX    int
		wantY    int
	}{
		{name: "top left", position: "top-left", wantX: 20, wantY: 20},
		{name: "top right", position: "top-right", wantX: 764, wantY: 20},
		{name: "bottom left", position: "bottom-left", wantX: 20, wantY: 708},
		{name: "bottom right", position: "bottom-right", wantX: 764, wantY: 708},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, w, h := aiLabelPlacement(1000, 800, 300, 100, tt.position)
			if x != tt.wantX || y != tt.wantY || w != 216 || h != 72 {
				t.Fatalf("got (%d,%d) %dx%d, want (%d,%d) 216x72", x, y, w, h, tt.wantX, tt.wantY)
			}
		})
	}
}

func TestAILabelPlacementCapsWideLabel(t *testing.T) {
	x, y, w, h := aiLabelPlacement(200, 100, 7459, 2363, "bottom-right")
	if x != 146 || y != 81 || w != 51 || h != 16 {
		t.Fatalf("got (%d,%d) %dx%d, want (146,81) 51x16", x, y, w, h)
	}
}
