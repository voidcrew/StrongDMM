package pmap

import (
	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/canvas"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/util"
)

// EditContext changes only presentation. Tools, snapshots and saving retain the
// active source document's native coordinates and instance identities.
type EditContext struct {
	View          *dmmap.Dmm
	Offset        util.Point
	StackID       string
	Refresh       func()
	BeforeHistory func()
	Filter        func(string) bool
	Editable      map[uint64]*dmminstance.Instance
}

func (p *PaneMap) SetEditContext(context *EditContext) {
	p.context = context
	p.canvasControl.AtCursor = context != nil
	if context != nil {
		p.canvasState.Offset = context.Offset
		p.canvas.ClearColor = canvas.Color{R: 0.065, G: 0.075, B: 0.09, A: 1}
	}
}

func (p *PaneMap) ViewDmm() *dmmap.Dmm {
	if p.context != nil && p.context.View != nil {
		return p.context.View
	}
	return p.dmm
}

func (p *PaneMap) CommandStackId() string {
	if p.context != nil {
		return p.context.StackID
	}
	return p.dmm.Path.Absolute
}

func (p *PaneMap) InContext() bool { return p.context != nil }
func (p *PaneMap) BeforeHistory() {
	if p.context != nil && p.context.BeforeHistory != nil {
		p.context.BeforeHistory()
	}
}

func (p *PaneMap) MapToView(coord util.Point) util.Point {
	if p.context != nil {
		coord.X += p.context.Offset.X
		coord.Y += p.context.Offset.Y
	}
	return coord
}

func (p *PaneMap) Refresh(coords []util.Point) {
	if p.context != nil && p.context.Refresh != nil {
		p.context.Refresh()
		return
	}
	p.canvas.Render().UpdateBucketV(p.dmm, p.activeLevel, coords)
}

func (p *PaneMap) RenderContext() { p.canvas.Render().ReplaceBucket(p.ViewDmm(), p.activeLevel) }

func (p *PaneMap) FitView() {
	p.centered = false
}
