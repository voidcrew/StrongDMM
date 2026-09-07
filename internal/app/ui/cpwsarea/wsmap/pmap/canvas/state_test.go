package canvas

import (
	"sdmm/internal/util"
	"testing"
)

func TestProjectedMouseUsesSourceCoordinates(t *testing.T) {
	s := NewState(4, 5, 32)
	s.Offset = util.Point{X: 11, Y: 7}
	s.SetMousePosition(12*32+16, 9*32+16, 1)
	if got := s.HoveredTile(); got != (util.Point{X: 2, Y: 3, Z: 1}) {
		t.Fatalf("wrong local tile: %v", got)
	}
	if s.HoverOutOfBounds() {
		t.Fatal("valid source tile rejected")
	}
	if s.RelMouseX() != 400 || s.RelMouseY() != 304 {
		t.Fatal("hit testing lost scene coordinates")
	}
	s.SetMousePosition(10*32+16, 9*32+16, 1)
	if !s.HoverOutOfBounds() {
		t.Fatal("ghost hull tile accepted as editable")
	}
}
