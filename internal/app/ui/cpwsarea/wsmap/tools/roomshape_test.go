package tools

import (
	"sdmm/internal/util"
	"sort"
	"testing"
)

func at(x, y int) util.Point { return util.Point{X: x, Y: y, Z: 1} }

func sortedTiles(tiles []util.Point) []util.Point {
	sort.Slice(tiles, func(i, j int) bool {
		if tiles[i].Y != tiles[j].Y {
			return tiles[i].Y < tiles[j].Y
		}
		return tiles[i].X < tiles[j].X
	})
	return tiles
}

func shapeFixture(t *testing.T) *ToolRoomShape {
	t.Helper()
	previous := Selected().Name()
	previousShift, previousNow := shiftDown, now
	shiftDown = func() bool { return false }
	now = func() float64 { return 1 }
	tool := SetSelected(TNRoomShape).(*ToolRoomShape)
	ClearRoomShape()
	tool.Accept = nil
	tool.rejectedAt, tool.reason = 0, ""
	t.Cleanup(func() {
		ClearRoomShape()
		tool.Accept = nil
		shiftDown, now = previousShift, previousNow
		SetSelected(previous)
	})
	return tool
}

func TestRoomShapeClickTogglesAndDragPaintsOneWay(t *testing.T) {
	tool := shapeFixture(t)
	tool.onStart(at(2, 2))
	tool.onStop(at(2, 2))
	if !tool.Has(at(2, 2)) {
		t.Fatal("click did not add the tile")
	}
	tool.onStart(at(2, 2))
	tool.onStop(at(2, 2))
	if tool.Has(at(2, 2)) {
		t.Fatal("second click did not remove the tile")
	}
	// An adding stroke keeps adding, even across tiles that were already in.
	SetRoomShapeTiles([]util.Point{at(4, 2)})
	tool.onStart(at(3, 2))
	tool.onMove(at(4, 2))
	tool.onMove(at(5, 2))
	tool.onStop(at(5, 2))
	got := sortedTiles(RoomShapeTiles())
	if len(got) != 3 || got[0] != at(3, 2) || got[1] != at(4, 2) || got[2] != at(5, 2) {
		t.Fatalf("adding stroke toggled mid-drag: %v", got)
	}
	// A removing stroke starts on a set tile and only removes.
	tool.onStart(at(4, 2))
	tool.onMove(at(5, 2))
	tool.onMove(at(6, 2))
	tool.onStop(at(6, 2))
	got = RoomShapeTiles()
	if len(got) != 1 || got[0] != at(3, 2) {
		t.Fatalf("removing stroke: %v", got)
	}
	if !tool.Stale() {
		t.Fatal("tool reports an unfinished stroke")
	}
}

func TestRoomShapeRectangles(t *testing.T) {
	tool := shapeFixture(t)
	shiftDown = func() bool { return true }
	tool.onStart(at(5, 5))
	tool.onMove(at(2, 3))
	if len(RoomShapeTiles()) != 0 {
		t.Fatal("rectangle applied before the drag ended")
	}
	if tool.Stale() {
		t.Fatal("dragging rectangle reported stale")
	}
	tool.onStop(at(2, 3))
	if n := len(RoomShapeTiles()); n != 12 {
		t.Fatalf("shift rectangle added %d tiles, want 12", n)
	}
	shiftDown = func() bool { return false }
	tool.setAltBehaviour(true)
	tool.onStart(at(3, 3))
	tool.onMove(at(4, 4))
	tool.onStop(at(4, 4))
	tool.setAltBehaviour(false)
	got := RoomShapeTiles()
	if len(got) != 8 {
		t.Fatalf("alt rectangle left %d tiles, want 8", len(got))
	}
	for _, tile := range got {
		if tile.X >= 3 && tile.X <= 4 && tile.Y >= 3 && tile.Y <= 4 {
			t.Fatalf("alt rectangle kept %v", tile)
		}
	}
	AddRoomShapeRect(at(3, 3), at(4, 4))
	if len(RoomShapeTiles()) != 12 {
		t.Fatal("AddRoomShapeRect did not restore the box")
	}
}

func TestRoomShapeAcceptRejectsAndReports(t *testing.T) {
	tool := shapeFixture(t)
	SetRoomShapeAccept(func(p util.Point) (bool, string) {
		if p.X > 3 {
			return false, "outside"
		}
		return true, ""
	})
	tool.onStart(at(4, 1))
	tool.onStop(at(4, 1))
	if len(RoomShapeTiles()) != 0 || RoomShapeRejection() != "outside" {
		t.Fatalf("rejected click was accepted: %v %q", RoomShapeTiles(), RoomShapeRejection())
	}
	if tool.rejected != at(4, 1) || tool.rejectedAt != 1 {
		t.Fatal("rejection was not recorded for the canvas flash")
	}
	tool.onStart(at(1, 1))
	tool.onMove(at(2, 1))
	tool.onMove(at(4, 1))
	tool.onStop(at(4, 1))
	got := sortedTiles(RoomShapeTiles())
	if len(got) != 2 || got[0] != at(1, 1) || got[1] != at(2, 1) {
		t.Fatalf("stroke did not skip rejected tiles: %v", got)
	}
	if RoomShapeRejection() != "outside" {
		t.Fatal("rejection during the stroke was not kept for the panel")
	}
	tool.onStart(at(3, 1))
	tool.onStop(at(3, 1))
	if RoomShapeRejection() != "" {
		t.Fatal("an accepted tile did not clear the rejection")
	}
	AddRoomShapeRect(at(1, 2), at(5, 2))
	if n := len(RoomShapeTiles()); n != 6 {
		t.Fatalf("imported rectangle ignored the acceptance hook: %d tiles", n)
	}
	// Removing never consults the hook.
	SetRoomShapeAccept(func(util.Point) (bool, string) { return false, "never" })
	tool.onStart(at(1, 1))
	tool.onStop(at(1, 1))
	if tool.Has(at(1, 1)) {
		t.Fatal("removal was blocked by the acceptance hook")
	}
}

func TestRoomShapeSurvivesToolSwitch(t *testing.T) {
	shapeFixture(t)
	SetRoomShapeTiles([]util.Point{at(1, 1), at(2, 1)})
	SetSelected(TNGrab)
	SetSelected(TNRoomShape)
	if len(RoomShapeTiles()) != 2 {
		t.Fatal("switching tools lost the room tiles")
	}
	ClearRoomShape()
	if len(RoomShapeTiles()) != 0 {
		t.Fatal("clear kept tiles")
	}
}
