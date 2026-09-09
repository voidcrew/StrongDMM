package render

import (
	"github.com/go-gl/gl/v3.3-core/gl"
	"sdmm/internal/app/render/brush"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/mappreview"
)

func (r *Render) SetPreviewLighting(lighting *mappreview.Lighting) { r.previewLighting = lighting }

func (r *Render) drawPreviewLighting(width, height float32) {
	l := r.previewLighting
	if l == nil {
		return
	}
	bounds := r.viewportBounds(width, height)
	size := float32(dmmap.WorldIconSize)
	for y := max(0, int(bounds.Y1/size)); y < min(l.Height, int(bounds.Y2/size)+1); y++ {
		for x := max(0, int(bounds.X1/size)); x < min(l.Width, int(bounds.X2/size)+1); x++ {
			colors := l.Tile(x, y)
			brush.RectGradient(float32(x)*size, float32(y)*size, float32(x+1)*size, float32(y+1)*size, colors)
		}
	}
	gl.BlendFunc(gl.DST_COLOR, gl.ZERO)
	brush.Draw(width, height, r.Camera.ShiftX, r.Camera.ShiftY, r.Camera.Scale)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
}
