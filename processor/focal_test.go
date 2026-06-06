package processor

import "testing"

// Tag-free: focalWindow is pure integer math, so it runs on every local
// `go test ./...` without libvips. It locks the crop-window placement that
// focalCrop (vips side) relies on.

func TestFocalWindow(t *testing.T) {
	const set = true
	cases := []struct {
		name                 string
		rw, rh, tw, th       int
		fx, fy               float64
		wantL, wantT, wantCW int
		wantCH               int
	}{
		{
			// 4000×3000 cover-scaled to fill 1080×1080 → 1440×1080, focal high (y=0.2).
			// x centres at 720→left 180 (in range); y window would start at -324, so it
			// clamps to the top edge (0). Matches the plan's worked example.
			name: "top-pinned when focal near top",
			rw:   1440, rh: 1080, tw: 1080, th: 1080,
			fx: 0.5, fy: 0.2,
			wantL: 180, wantT: 0, wantCW: 1080, wantCH: 1080,
		},
		{
			// Dead-centre focal on a wide source: window centres horizontally.
			name: "centred focal",
			rw:   2000, rh: 1000, tw: 1000, th: 1000,
			fx: 0.5, fy: 0.5,
			wantL: 500, wantT: 0, wantCW: 1000, wantCH: 1000,
		},
		{
			// Focal at the far right clamps to the right edge (left = rw-cw).
			name: "right-clamped",
			rw:   2000, rh: 1000, tw: 1000, th: 1000,
			fx: 1.0, fy: 0.5,
			wantL: 1000, wantT: 0, wantCW: 1000, wantCH: 1000,
		},
		{
			// Focal at the far left clamps to 0.
			name: "left-clamped",
			rw:   2000, rh: 1000, tw: 1000, th: 1000,
			fx: 0.0, fy: 0.5,
			wantL: 0, wantT: 0, wantCW: 1000, wantCH: 1000,
		},
		{
			// Degenerate: target larger than the (resized) source. Window clamps to
			// the source dims and stays at the origin rather than going negative.
			name: "target larger than source",
			rw:   500, rh: 400, tw: 800, th: 800,
			fx: 0.5, fy: 0.5,
			wantL: 0, wantT: 0, wantCW: 500, wantCH: 400,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l, top, cw, ch := focalWindow(c.rw, c.rh, c.tw, c.th, FocalPoint{X: c.fx, Y: c.fy, Set: set})
			if l != c.wantL || top != c.wantT || cw != c.wantCW || ch != c.wantCH {
				t.Errorf("focalWindow = (l=%d,t=%d,cw=%d,ch=%d), want (l=%d,t=%d,cw=%d,ch=%d)",
					l, top, cw, ch, c.wantL, c.wantT, c.wantCW, c.wantCH)
			}
			// The window must always sit fully inside the source.
			if l < 0 || top < 0 || l+cw > c.rw || top+ch > c.rh {
				t.Errorf("window (l=%d,t=%d,cw=%d,ch=%d) escapes source %dx%d", l, top, cw, ch, c.rw, c.rh)
			}
		})
	}
}
