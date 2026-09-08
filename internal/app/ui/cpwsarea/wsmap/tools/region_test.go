package tools

import (
	"sdmm/internal/util"
	"testing"
)

func TestRegionRequiresCompletedDrag(t *testing.T) {
	previous := Selected().Name()
	defer SetSelected(previous)
	tool := SetSelected(TNRegion).(*ToolRegion)
	tool.OnDeselect()
	if _, _, ok := RegionBounds(); ok {
		t.Fatal("empty region was accepted")
	}
	tool.onStart(util.Point{X: 12, Y: 9, Z: 1})
	tool.onMove(util.Point{X: 3, Y: 4, Z: 1})
	if _, _, ok := RegionBounds(); ok {
		t.Fatal("unfinished drag was accepted")
	}
	// No editor is installed: region selection must not mutate a map.
	tool.onStop(util.Point{})
	lo, hi, ok := RegionBounds()
	if !ok || lo != (util.Point{X: 3, Y: 4, Z: 1}) || hi != (util.Point{X: 12, Y: 9, Z: 1}) {
		t.Fatalf("backwards drag: %v %v %v", lo, hi, ok)
	}
	// Starting inside the old rectangle selects anew instead of moving tiles.
	tool.onStart(util.Point{X: 5, Y: 5, Z: 1})
	tool.onMove(util.Point{X: 7, Y: 7, Z: 1})
	tool.onStop(util.Point{})
	lo, hi, ok = RegionBounds()
	if !ok || lo.X != 5 || lo.Y != 5 || hi.X != 7 || hi.Y != 7 {
		t.Fatal("second drag reused old bounds")
	}
	SetSelected(TNAdd)
	if _, _, ok := RegionBounds(); ok {
		t.Fatal("deselected region was accepted")
	}
	SetSelected(TNRegion)
	if _, _, ok := RegionBounds(); ok {
		t.Fatal("stale region survived tool switch")
	}
}
