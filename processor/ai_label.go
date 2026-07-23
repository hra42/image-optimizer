package processor

// AILabel describes an optional visible AI disclosure composited onto image
// outputs. It contains plain data so non-vips builds can parse and test it.
type AILabel struct {
	Style    string
	Color    string
	Position string
}

// Enabled reports whether a label was selected. Upload validation guarantees
// the remaining fields are valid whenever Style is non-empty.
func (l AILabel) Enabled() bool {
	return l.Style != ""
}

// aiLabelPlacement returns a scaled stamp size and its top-left output
// coordinate. Labels target 9% of the short edge, never exceed 42% of the image
// width, and retain a 2.5% inset so they remain useful across preset sizes.
func aiLabelPlacement(imageW, imageH, stampW, stampH int, position string) (x, y, width, height int) {
	if imageW <= 0 || imageH <= 0 || stampW <= 0 || stampH <= 0 {
		return 0, 0, 0, 0
	}

	short := imageW
	if imageH < short {
		short = imageH
	}
	margin := clampInt(roundInt(float64(short)*0.025), 1, 48)
	targetH := clampInt(roundInt(float64(short)*0.09), 16, 120)

	width = roundInt(float64(stampW) * float64(targetH) / float64(stampH))
	height = targetH
	maxW := clampInt(roundInt(float64(imageW)*0.42), 1, imageW-2*margin)
	if width > maxW {
		width = maxW
		height = roundInt(float64(stampH) * float64(width) / float64(stampW))
	}
	maxH := imageH - 2*margin
	if height > maxH {
		height = maxH
		width = roundInt(float64(stampW) * float64(height) / float64(stampH))
	}
	if width < 1 || height < 1 {
		return 0, 0, 0, 0
	}

	x, y = margin, margin
	if position == "top-right" || position == "bottom-right" {
		x = imageW - margin - width
	}
	if position == "bottom-left" || position == "bottom-right" {
		y = imageH - margin - height
	}
	return x, y, width, height
}

func roundInt(v float64) int {
	if v < 0 {
		return int(v - 0.5)
	}
	return int(v + 0.5)
}
