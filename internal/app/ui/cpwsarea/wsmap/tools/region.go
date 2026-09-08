package tools

import (
	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/overlay"
	"sdmm/internal/util"
)

// ToolRegion selects a rectangle without moving or changing its contents.
// Each drag starts a new selection, including drags inside the previous one.
type ToolRegion struct {
	tool
	start, end      util.Point
	dragging, ready bool
}

func (*ToolRegion) Name() string  { return TNRegion }
func (t *ToolRegion) Stale() bool { return !t.dragging }
func (t *ToolRegion) OnDeselect() { t.ready, t.dragging = false, false }
func (t *ToolRegion) onStart(p util.Point) {
	t.start, t.end = p, p
	t.dragging, t.ready = true, false
}
func (t *ToolRegion) onMove(p util.Point) {
	if t.dragging {
		t.end = p
	}
}
func (t *ToolRegion) onStop(util.Point) {
	if t.dragging {
		t.dragging, t.ready = false, true
	}
}
func (t *ToolRegion) bounds() (util.Point, util.Point) {
	return util.Point{X: min(t.start.X, t.end.X), Y: min(t.start.Y, t.end.Y), Z: t.start.Z},
		util.Point{X: max(t.start.X, t.end.X), Y: max(t.start.Y, t.end.Y), Z: t.start.Z}
}
func (t *ToolRegion) process() {
	if !t.dragging && !t.ready {
		return
	}
	lo, hi := t.bounds()
	ed.OverlayPushArea(util.Bounds{X1: float32(lo.X), Y1: float32(lo.Y), X2: float32(hi.X), Y2: float32(hi.Y)},
		overlay.ColorToolSelectTileFill, overlay.ColorToolSelectTileBorder)
}

// RegionBounds returns only a completed, explicit region selection. It never
// substitutes the last hovered tile when there is no selection.
func RegionBounds() (util.Point, util.Point, bool) {
	t, ok := Selected().(*ToolRegion)
	if !ok || !t.ready || t.dragging {
		return util.Point{}, util.Point{}, false
	}
	lo, hi := t.bounds()
	return lo, hi, true
}
