package mappreview

import (
	"testing"

	"sdmm/internal/dmapi/dmenv"
)

func TestExteriorLightIlluminatesHullWithoutLightingSealedRooms(t *testing.T) {
	for _, padding := range []int{0, 2} {
		for _, outside := range []string{"/turf/open/space", "/turf/template_noop", ""} {
			m := emptyMap(7+2*padding, 7+2*padding, 1)
			for _, tile := range m.Tiles {
				x, y := tile.Coord.X-padding, tile.Coord.Y-padding
				if x < 1 || x > 7 || y < 1 || y > 7 {
					if outside != "" {
						tile.InstancesAdd(prefab(outside, nil))
					}
				} else if x == 1 || x == 7 || y == 1 || y == 7 {
					tile.InstancesAdd(prefab("/turf/closed/wall", map[string]string{"opacity": "1"}))
				} else if x == 4 && y == 4 {
					tile.InstancesAdd(prefab("/turf/open/space", nil)) // A sealed interior pocket.
				} else {
					tile.InstancesAdd(prefab("/turf/open/floor", nil))
				}
			}
			before := m.Copy()
			dark, _ := buildLighting(m, true, false)
			scene := Build(m, &dmenv.Dme{}, Options{Lighting: true, ExteriorLight: true}, nil)
			lit := scene.Lighting
			// West-facing corners of the west hull wall receive exterior light.
			if c := lit.Tile(padding, padding+3); c[0][0] < .8 || c[2][0] < .8 || c[1] != (RGB{}) || c[3] != (RGB{}) {
				t.Fatalf("wrong exterior wall lighting (padding=%d outside=%s): %v", padding, outside, c)
			}
			for y := padding + 1; y < padding+6; y++ {
				for x := padding + 1; x < padding+6; x++ {
					if lit.Tile(x, y) != dark.Tile(x, y) {
						t.Fatalf("exterior light leaked inside at %d,%d", x, y)
					}
				}
			}
			if padding > 0 && lit.Tile(0, 0)[0][0] < .8 {
				t.Fatal("surrounding space remains unlit")
			}
			if scene.Lights != 0 {
				t.Fatal("exterior light counted as a mapped fixture")
			}
			for i, tile := range m.Tiles {
				if tile.Coord != before.Tiles[i].Coord || !tile.Instances().PrefabsEquals(before.Tiles[i].Instances()) {
					t.Fatal("exterior lighting changed the map")
				}
			}
		}
	}
}
