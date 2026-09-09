package mappreview

import (
	"regexp"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
)

var structureSpawnList = regexp.MustCompile(`^list\(\s*(/\w+(?:/\w+)*\s*(?:,\s*/\w+(?:/\w+)*\s*)*)\)$`)

// Resolve declarative window spawners before smoothing: their windows, rather
// than the editor markers, supply the neighboring walls' smoothing groups.
func initialMap(source *dmmap.Dmm, dme *dmenv.Dme, notes map[string]bool) *dmmap.Dmm {
	result := source.Copy()
	for _, tile := range result.Tiles {
		covered := false
		for _, instance := range tile.Instances() {
			p := instance.Prefab()
			if dm.IsPath(p.Path(), "/turf") {
				covered = p.Vars().IntV("underfloor_accessibility", 2) < 1
			}
		}
		var instances dmmap.Instances
		for _, instance := range tile.Instances() {
			p := instance.Prefab()
			if dm.IsPath(p.Path(), "/area/overmap_encounter/planetoid") && !dm.IsPath(p.Path(), "/area/overmap_encounter/planetoid/cave") {
				if definition := dme.Objects[p.Vars().ValueV("planet_type", "null")]; definition != nil && definition.Vars.IntV("definition_version", 0) == 1 {
					if environment := dme.Objects[definition.Vars.ValueV("environment", "null")]; environment != nil {
						fields := map[string]string{"static_lighting": "0", "ambient_lighting": "1", "base_lighting_color": environment.Vars.ValueV("light_color", `"#FFFFFF"`), "base_lighting_alpha": environment.Vars.ValueV("light_alpha", "255")}
						// A Planet Workshop preview already supplies its live draft's
						// area overrides. Keep those over the saved definition.
						for _, field := range p.Vars().Iterate() {
							delete(fields, field)
						}
						p = withVars(p, fields)
						instance.SetPrefab(p)
					}
				}
			}
			if covered && (dm.IsPath(p.Path(), "/obj/structure/cable") ||
				(dm.IsPath(p.Path(), "/obj/machinery/atmospherics/pipe") && p.Vars().IntV("hide", 0) != 0)) {
				continue
			}
			if dm.IsPath(p.Path(), "/obj/effect/spawner/structure/window") {
				if spawned := windowSpawns(p, dme); len(spawned) != 0 {
					for _, child := range spawned {
						instances = append(instances, dmminstance.New(tile.Coord, child))
					}
					continue
				}
				notes["Window spawner needs runtime setup: "+p.Path()] = true
			}
			// Closed doors move above floor objects during Initialize().
			if dm.IsPath(p.Path(), "/obj/machinery/door") && p.Vars().IntV("density", 0) != 0 {
				if layer, ok := p.Vars().Value("closingLayer"); ok && layer != "null" {
					instance.SetPrefab(withVars(p, map[string]string{"layer": layer}))
				}
			}
			instances = append(instances, instance)
		}
		tile.Set(instances)
	}
	return &result
}

func windowSpawns(p *dmmprefab.Prefab, dme *dmenv.Dme) []*dmmprefab.Prefab {
	// Hollow spawners alter their lists in Initialize() based on direction.
	if dm.IsPath(p.Path(), "/obj/effect/spawner/structure/window/hollow") {
		return nil
	}
	match := structureSpawnList.FindStringSubmatch(p.Vars().ValueV("spawn_list", ""))
	if match == nil {
		return nil
	}
	var result []*dmmprefab.Prefab
	for _, path := range strings.Split(match[1], ",") {
		path = strings.TrimSpace(path)
		object := dme.Objects[path]
		if object == nil || (!dm.IsPath(path, "/obj/structure/window") && !dm.IsPath(path, "/obj/structure/grille")) {
			return nil
		}
		result = append(result, dmmprefab.New(dmmprefab.IdStage, path, object.Vars))
	}
	return result
}

// Airlock frames have transparent holes for their material/fill overlays.
// Keep those overlays on the door's plane, layer, direction and pixel offsets.
func addAirlockOverlays(scene *dmmap.Dmm, hasState HasState) {
	if hasState == nil {
		return
	}
	for _, tile := range scene.Tiles {
		var overlays dmmap.Instances
		for _, instance := range tile.Instances() {
			p := instance.Prefab()
			if !dm.IsPath(p.Path(), "/obj/machinery/door/airlock") {
				continue
			}
			vars := p.Vars()
			frame := "closed"
			if vars.IntV("density", 1) == 0 {
				frame = "open"
			}
			icon := vars.TextV("icon", "")
			if state := vars.TextV("base_icon_state", "") + frame; hasState(icon, state) {
				p = withVars(p, map[string]string{"icon_state": strconv.Quote(state)})
				instance.SetPrefab(p)
			}
			material := vars.TextV("airlock_material", "")
			if vars.IntV("glass", 0) != 0 {
				material = "glass"
			}
			state := "fill_" + frame
			if material != "" {
				icon = vars.TextV("overlays_file", "")
				state = material + "_" + frame
			}
			if !hasState(icon, state) {
				continue
			}
			fields := map[string]string{
				"icon": "'" + icon + "'", "icon_state": strconv.Quote(state),
				"layer": strconv.FormatFloat(float64(vars.FloatV("layer", 0))+.001, 'f', -1, 64),
			}
			overlays = append(overlays, dmminstance.New(tile.Coord, withVars(p, fields)))
		}
		tile.Set(append(tile.Instances(), overlays...))
	}
}
