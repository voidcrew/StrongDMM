package tools

import (
	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap/overlay"
	"sdmm/internal/imguiext"
	"sdmm/internal/util"
)

type shapeMode int

const (
	shapePaintAdd shapeMode = iota
	shapePaintRemove
	shapeRectAdd
	shapeRectRemove
)

const rejectFlashSec = .6

// Hooks the workshop installs; kept as variables so tests can drive the tool
// without a window.
var (
	shiftDown = imguiext.IsShiftDown
	now       = func() float64 { return imgui.Time() }
)

// ToolRoomShape collects the tiles of an upgrade room on the hull, one tile at
// a time or by rectangles. It only exists for Ship Workshop: it never changes
// map content and is not offered in the ordinary tool bar.
//
// Click toggles a tile. A drag paints: the first tile decides whether the
// stroke adds or removes, and every tile passed gets that state. Shift+drag
// adds a rectangle, Alt+drag removes one.
type ToolRoomShape struct {
	tool
	tiles      map[util.Point]bool
	mode       shapeMode
	start, end util.Point
	dragging   bool
	// Accept vets additions; a rejection is shown briefly on the canvas and
	// its reason is kept for the panel.
	Accept     func(util.Point) (bool, string)
	rejected   util.Point
	rejectedAt float64
	reason     string
}

func newRoomShape() *ToolRoomShape { return &ToolRoomShape{tiles: map[util.Point]bool{}} }

func (*ToolRoomShape) Name() string  { return TNRoomShape }
func (t *ToolRoomShape) Stale() bool { return !t.dragging }
func (t *ToolRoomShape) OnDeselect() { t.dragging = false }
func (t *ToolRoomShape) Tiles() []util.Point {
	tiles := make([]util.Point, 0, len(t.tiles))
	for tile, in := range t.tiles {
		if in {
			tiles = append(tiles, tile)
		}
	}
	return tiles
}
func (t *ToolRoomShape) Has(p util.Point) bool { return t.tiles[p] }

func (t *ToolRoomShape) add(p util.Point) bool {
	if t.tiles[p] {
		return true
	}
	if t.Accept != nil {
		if ok, reason := t.Accept(p); !ok {
			t.rejected, t.rejectedAt, t.reason = p, now(), reason
			return false
		}
	}
	t.tiles[p] = true
	t.reason = ""
	return true
}

func (t *ToolRoomShape) onStart(p util.Point) {
	t.dragging = true
	t.start, t.end = p, p
	switch {
	case shiftDown():
		t.mode = shapeRectAdd
	case t.altBehaviour:
		t.mode = shapeRectRemove
	case t.tiles[p]:
		t.mode = shapePaintRemove
		delete(t.tiles, p)
	default:
		t.mode = shapePaintAdd
		t.add(p)
	}
}

func (t *ToolRoomShape) onMove(p util.Point) {
	if !t.dragging {
		return
	}
	switch t.mode {
	case shapePaintAdd:
		t.add(p)
	case shapePaintRemove:
		delete(t.tiles, p)
	default:
		t.end = p
	}
}

func (t *ToolRoomShape) onStop(util.Point) {
	if !t.dragging {
		return
	}
	t.dragging = false
	if t.mode != shapeRectAdd && t.mode != shapeRectRemove {
		return
	}
	lo, hi := t.bounds()
	for y := lo.Y; y <= hi.Y; y++ {
		for x := lo.X; x <= hi.X; x++ {
			p := util.Point{X: x, Y: y, Z: lo.Z}
			if t.mode == shapeRectAdd {
				t.add(p)
			} else {
				delete(t.tiles, p)
			}
		}
	}
}

func (t *ToolRoomShape) bounds() (util.Point, util.Point) {
	return util.Point{X: min(t.start.X, t.end.X), Y: min(t.start.Y, t.end.Y), Z: t.start.Z},
		util.Point{X: max(t.start.X, t.end.X), Y: max(t.start.Y, t.end.Y), Z: t.start.Z}
}

func (t *ToolRoomShape) process() {
	if ed == nil {
		return
	}
	for tile, in := range t.tiles {
		if in {
			ed.OverlayPushTile(tile, overlay.ColorRoomShapeFill, overlay.ColorEmpty)
		}
	}
	if t.dragging && (t.mode == shapeRectAdd || t.mode == shapeRectRemove) {
		lo, hi := t.bounds()
		fill, border := overlay.ColorRoomShapeFill, overlay.ColorRoomShapeBorder
		if t.mode == shapeRectRemove {
			fill, border = overlay.ColorToolDeleteAltTileFill, overlay.ColorToolDeleteInstance
		}
		ed.OverlayPushArea(util.Bounds{X1: float32(lo.X), Y1: float32(lo.Y), X2: float32(hi.X), Y2: float32(hi.Y)}, fill, border)
	}
	if t.rejectedAt > 0 {
		if delta := now() - t.rejectedAt; delta < rejectFlashSec {
			c := overlay.ColorRoomShapeReject
			ed.OverlayPushTile(t.rejected, util.MakeColor(c.R(), c.G(), c.B(), c.A()*float32(1-delta/rejectFlashSec)), overlay.ColorEmpty)
		} else {
			t.rejectedAt = 0
		}
	}
}

// RoomShapeTiles lists the tiles collected so far, in the hull's coordinates.
func RoomShapeTiles() []util.Point { return roomShape().Tiles() }

func SetRoomShapeTiles(tiles []util.Point) {
	t := roomShape()
	t.tiles = map[util.Point]bool{}
	for _, tile := range tiles {
		t.tiles[tile] = true
	}
	t.reason = ""
}

// AddRoomShapeRect adds a rectangle, honouring the acceptance hook.
func AddRoomShapeRect(lo, hi util.Point) {
	t := roomShape()
	for y := lo.Y; y <= hi.Y; y++ {
		for x := lo.X; x <= hi.X; x++ {
			t.add(util.Point{X: x, Y: y, Z: lo.Z})
		}
	}
}

func ClearRoomShape() { SetRoomShapeTiles(nil) }

// SetRoomShapeAccept installs the workshop's check for tiles being added.
func SetRoomShapeAccept(accept func(util.Point) (bool, string)) { roomShape().Accept = accept }

// RoomShapeRejection reports why the last click was refused, until a tile is accepted.
func RoomShapeRejection() string { return roomShape().reason }

func roomShape() *ToolRoomShape { return tools[TNRoomShape].(*ToolRoomShape) }
