// Package mappreview derives a display-only scene from mapped tg appearances.
// It does not execute DM initialization or modify the source document.
package mappreview

import (
	"fmt"
	"sort"
	"strconv"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

type Options struct {
	Smoothing, Lighting, PoweredFixtures, ExteriorLight bool
}

type Scene struct {
	Map              *dmmap.Dmm
	Lighting         *Lighting
	Smoothed, Lights int
	Notes            []string
}

// HasState checks exact DMI state names; a fallback state is not a match.
type HasState func(icon, state string) bool

func Snapshot(source *dmmap.Dmm, z int) *dmmap.Dmm {
	result := &dmmap.Dmm{Name: source.Name, MaxX: source.MaxX, MaxY: source.MaxY, MaxZ: 1}
	z = max(1, min(z, source.MaxZ))
	for y := 1; y <= source.MaxY; y++ {
		for x := 1; x <= source.MaxX; x++ {
			tile := source.GetTile(util.Point{X: x, Y: y, Z: z}).Copy()
			tile.Coord.Z = 1
			instances := make(dmmap.Instances, 0, len(tile.Instances()))
			for _, instance := range tile.Instances() {
				instances = append(instances, instance.CopyAt(tile.Coord))
			}
			tile.Set(instances)
			result.Tiles = append(result.Tiles, &tile)
		}
	}
	return result
}

func Build(source *dmmap.Dmm, dme *dmenv.Dme, options Options, hasState HasState) *Scene {
	notes := map[string]bool{}
	source = initialMap(source, dme, notes)
	copy := source.Copy()
	scene := &Scene{Map: &copy}
	if options.Smoothing {
		rules := loadRules(dme.RootDir)
		for _, tile := range scene.Map.Tiles {
			for _, instance := range tile.Instances() {
				prefab := instance.Prefab()
				vars := prefab.Vars()
				flags := vars.IntV("smoothing_flags", 0)
				if flags&(rules.bitmask|rules.cardinals) == 0 {
					continue
				}
				if flags&(rules.borderObject|rules.procFilter) != 0 {
					notes["Custom smoothing needs runtime code: "+prefab.Path()] = true
					continue
				}
				base := vars.TextV("base_icon_state", "")
				icon := vars.TextV("icon", "")
				if base == "" {
					continue
				}
				if dm.IsPath(prefab.Path(), "/turf/open/floor") && (vars.IntV("broken", 0) != 0 || vars.IntV("burnt", 0) != 0) {
					continue
				}
				junction := smoothJunction(source, tile.Coord, prefab, rules)
				state := fmt.Sprintf("%s-%d", base, junction)
				diagonal := flags&rules.diagonal != 0 && dm.IsPath(prefab.Path(), "/turf/closed") && diagonalJunction(junction)
				if diagonal && hasState(icon, state+"-d") {
					state += "-d"
				} else {
					diagonal = false
				}
				if !hasState(icon, state) {
					notes["Missing smoothed sprite: "+icon+" / "+state] = true
					continue
				}
				if diagonal {
					underlay := diagonalUnderlay(source, dme, tile.Coord, prefab, junction)
					if underlay != nil {
						tile.Set(append(tile.Instances(), dmminstance.New(tile.Coord, underlay)))
					}
				}
				instance.SetPrefab(withVars(prefab, map[string]string{"icon_state": strconv.Quote(state), "smoothing_junction": strconv.Itoa(junction)}))
				scene.Smoothed++
			}
		}
	}
	if options.Lighting {
		scene.Lighting, scene.Lights = buildLighting(source, options.PoweredFixtures, options.ExteriorLight)
	}
	addAirlockOverlays(scene.Map, hasState)
	for note := range notes {
		scene.Notes = append(scene.Notes, note)
	}
	sort.Strings(scene.Notes)
	return scene
}

func withVars(prefab *dmmprefab.Prefab, fields map[string]string) *dmmprefab.Prefab {
	vars := dmvars.FromParent(prefab.Vars())
	for name, value := range fields {
		vars = dmvars.Set(vars, name, value)
	}
	return dmmprefab.New(dmmprefab.IdStage, prefab.Path(), vars)
}

func Visible(prefab *dmmprefab.Prefab) bool {
	return !dm.IsPath(prefab.Path(), "/area") && prefab.Vars().IntV("invisibility", 0) == 0
}
