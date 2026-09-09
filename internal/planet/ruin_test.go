package planet

import (
	"testing"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func TestPlaceRuinPreservesNoopsOverridesAndSource(t *testing.T) {
	prefab := func(path string) *dmmprefab.Prefab {
		return dmmprefab.New(0, path, &dmvars.Variables{})
	}
	background := &dmmap.Dmm{Name: "Planet", MaxX: 4, MaxY: 4, MaxZ: 1}
	for y := 1; y <= 4; y++ {
		for x := 1; x <= 4; x++ {
			tile := &dmmap.Tile{Coord: util.Point{X: x, Y: y, Z: 1}}
			tile.InstancesSet(dmmdata.Prefabs{prefab("/turf/open/grass"), prefab("/area/planet"), prefab("/obj/tree")})
			background.Tiles = append(background.Tiles, tile)
		}
	}
	ruin := &dmmap.Dmm{Name: "Ruin", MaxX: 2, MaxY: 1, MaxZ: 1}
	for x := 1; x <= 2; x++ {
		tile := &dmmap.Tile{Coord: util.Point{X: x, Y: 1, Z: 1}}
		if x == 1 {
			object := dmmprefab.New(0, "/obj/chair", dmvars.Set(&dmvars.Variables{}, "dir", "4"))
			tile.InstancesSet(dmmdata.Prefabs{object, prefab("/turf/closed/rock"), prefab("/area/ruin")})
		} else {
			tile.InstancesSet(dmmdata.Prefabs{prefab("/turf/template_noop"), prefab("/area/template_noop")})
		}
		ruin.Tiles = append(ruin.Tiles, tile)
	}
	result, err := PlaceRuin(background, ruin, 1)
	if err != nil {
		t.Fatal(err)
	}
	first := result.GetTile(util.Point{X: 2, Y: 2, Z: 1}).Instances()
	if len(first) != 3 || first[0].Prefab().Path() != "/obj/chair" || first[0].Prefab().Vars().IntV("dir", 0) != 4 || first[0].Coord() != (util.Point{X: 2, Y: 2, Z: 1}) {
		t.Fatal("lost mapped content, direction or translated coordinates")
	}
	second := result.GetTile(util.Point{X: 3, Y: 2, Z: 1}).Instances()
	if len(second) != 2 || second[0].Prefab().Path() != "/turf/open/grass" || second[1].Prefab().Path() != "/area/planet" {
		t.Fatal("noop tile did not preserve host terrain and area")
	}
	if len(background.GetTile(util.Point{X: 2, Y: 2, Z: 1}).Instances()) != 3 || ruin.Tiles[0].Instances()[0].Coord().X != 1 {
		t.Fatal("preview changed a source map")
	}
	if _, err = PlaceRuin(background, ruin, 2); err == nil {
		t.Fatal("accepted an invalid ruin level")
	}
}
