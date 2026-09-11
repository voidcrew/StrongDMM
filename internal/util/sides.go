package util

// Sides marks the edges of a tile that face away from a tile set.
type Sides struct {
	North, East, South, West bool
}

func (s Sides) Any() bool { return s.North || s.East || s.South || s.West }

// OuterSides returns, for every tile in the set, the edges that border a tile
// outside the set. Drawing only those edges outlines any shape, including
// L-shapes and holes, without lines between neighbouring tiles.
func OuterSides(tiles map[Point]bool) map[Point]Sides {
	sides := map[Point]Sides{}
	for tile, in := range tiles {
		if !in {
			continue
		}
		s := Sides{
			North: !tiles[Point{X: tile.X, Y: tile.Y + 1, Z: tile.Z}],
			East:  !tiles[Point{X: tile.X + 1, Y: tile.Y, Z: tile.Z}],
			South: !tiles[Point{X: tile.X, Y: tile.Y - 1, Z: tile.Z}],
			West:  !tiles[Point{X: tile.X - 1, Y: tile.Y, Z: tile.Z}],
		}
		if s.Any() {
			sides[tile] = s
		}
	}
	return sides
}
