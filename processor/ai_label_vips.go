//go:build vips

package processor

import (
	"embed"
	"fmt"

	"github.com/davidbyttow/govips/v2/vips"
)

//go:embed stamps/*.png
var aiLabelAssets embed.FS

func applyAILabel(img *vips.ImageRef, label AILabel) error {
	if !label.Enabled() {
		return nil
	}
	data, err := aiLabelAssets.ReadFile(fmt.Sprintf("stamps/%s-%s.png", label.Style, label.Color))
	if err != nil {
		return fmt.Errorf("load AI label: %w", err)
	}
	stamp, err := vips.NewImageFromBuffer(data)
	if err != nil {
		return fmt.Errorf("decode AI label: %w", err)
	}
	defer stamp.Close()

	x, y, width, height := aiLabelPlacement(
		img.Width(), img.Height(), stamp.Width(), stamp.Height(), label.Position,
	)
	if width == 0 || height == 0 {
		return nil
	}
	if err := stamp.ResizeWithVScale(
		float64(width)/float64(stamp.Width()),
		float64(height)/float64(stamp.Height()),
		vips.KernelLanczos3,
	); err != nil {
		return fmt.Errorf("resize AI label: %w", err)
	}
	if err := img.Composite(stamp, vips.BlendModeOver, x, y); err != nil {
		return fmt.Errorf("composite AI label: %w", err)
	}
	return nil
}
