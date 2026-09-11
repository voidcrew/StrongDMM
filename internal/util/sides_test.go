package util

import "testing"

func TestOuterSidesOutlinesShapes(t *testing.T) {
	at := func(x, y int) Point { return Point{X: x, Y: y, Z: 1} }
	single := OuterSides(map[Point]bool{at(3, 3): true})
	if single[at(3, 3)] != (Sides{true, true, true, true}) {
		t.Fatalf("single tile: %+v", single[at(3, 3)])
	}
	// A 2x2 block outlines with two sides per corner and no inner edges.
	block := map[Point]bool{at(1, 1): true, at(2, 1): true, at(1, 2): true, at(2, 2): true}
	got := OuterSides(block)
	if len(got) != 4 || got[at(1, 1)] != (Sides{South: true, West: true}) || got[at(2, 2)] != (Sides{North: true, East: true}) {
		t.Fatalf("block: %+v", got)
	}
	// L-shape: the inner corner tile has exactly one outer edge on each free side.
	l := map[Point]bool{at(1, 1): true, at(2, 1): true, at(3, 1): true, at(1, 2): true, at(1, 3): true}
	got = OuterSides(l)
	if got[at(1, 1)] != (Sides{South: true, West: true}) || got[at(2, 1)] != (Sides{North: true, South: true}) || got[at(1, 2)] != (Sides{East: true, West: true}) {
		t.Fatalf("L: %+v", got)
	}
	// Interior tiles of a filled 3x3 have no outer edges and are omitted.
	full := map[Point]bool{}
	for y := 1; y <= 3; y++ {
		for x := 1; x <= 3; x++ {
			full[at(x, y)] = true
		}
	}
	got = OuterSides(full)
	if _, ok := got[at(2, 2)]; ok || len(got) != 8 {
		t.Fatalf("3x3: %d edged tiles, centre present %v", len(got), ok)
	}
	// False entries count as outside.
	got = OuterSides(map[Point]bool{at(1, 1): true, at(2, 1): false})
	if got[at(1, 1)] != (Sides{true, true, true, true}) {
		t.Fatalf("false neighbour treated as inside: %+v", got[at(1, 1)])
	}
}
