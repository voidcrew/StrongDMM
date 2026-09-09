package planet

import (
	"fmt"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"
)

// PlaceRuin creates a display-only composite, centered in the generated terrain.
// Noop turf/area placeholders retain their host; mapped atoms keep their vars.
func PlaceRuin(terrain, ruin *dmmap.Dmm, level int) (*dmmap.Dmm, error) {
	if terrain == nil || ruin == nil || ruin.MaxX > terrain.MaxX || ruin.MaxY > terrain.MaxY || level < 1 || level > ruin.MaxZ {
		return nil, fmt.Errorf("The ruin does not fit this terrain preview.")
	}
	result := terrain.Copy()
	dx, dy := (terrain.MaxX-ruin.MaxX)/2, (terrain.MaxY-ruin.MaxY)/2
	for y := 1; y <= ruin.MaxY; y++ {
		for x := 1; x <= ruin.MaxX; x++ {
			source := ruin.GetTile(util.Point{X: x, Y: y, Z: level})
			target := result.GetTile(util.Point{X: x + dx, Y: y + dy, Z: 1})
			keepTurf, keepArea := true, true
			for _, instance := range source.Instances() {
				path := instance.Prefab().Path()
				if dm.IsPath(path, "/turf") && path != "/turf/template_noop" {
					keepTurf = false
				}
				if dm.IsPath(path, "/area") && path != "/area/template_noop" {
					keepArea = false
				}
			}
			var instances dmmap.Instances
			for _, instance := range target.Instances() {
				path := instance.Prefab().Path()
				if (keepTurf && dm.IsPath(path, "/turf")) || (keepArea && dm.IsPath(path, "/area")) {
					instances = append(instances, instance)
				}
			}
			for _, instance := range source.Instances() {
				path := instance.Prefab().Path()
				if path != "/turf/template_noop" && path != "/area/template_noop" {
					instances = append(instances, instance.CopyAt(target.Coord))
				}
			}
			target.Set(instances)
		}
	}
	result.Name = ruin.Name + " on " + terrain.Name
	return &result, nil
}
