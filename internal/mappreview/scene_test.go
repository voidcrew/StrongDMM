package mappreview

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func prefab(path string, fields map[string]string) *dmmprefab.Prefab {
	v := &dmvars.Variables{}
	for name, value := range fields {
		v = dmvars.Set(v, name, value)
	}
	return dmmprefab.New(dmmprefab.IdStage, path, v)
}

func emptyMap(w, h, levels int) *dmmap.Dmm {
	m := &dmmap.Dmm{MaxX: w, MaxY: h, MaxZ: levels}
	for z := 1; z <= levels; z++ {
		for y := 1; y <= h; y++ {
			for x := 1; x <= w; x++ {
				m.Tiles = append(m.Tiles, &dmmap.Tile{Coord: util.Point{X: x, Y: y, Z: z}})
			}
		}
	}
	return m
}

func at(m *dmmap.Dmm, x, y int) *dmmap.Tile { return m.GetTile(util.Point{X: x, Y: y, Z: 1}) }

func TestSmoothingConnections(t *testing.T) {
	rules := loadRules("")
	wall := prefab("/turf/closed/wall", map[string]string{"smoothing_flags": "1", "smoothing_groups": `"1,"`, "canSmoothWith": `"1,-2,"`})
	m := emptyMap(3, 3, 1)
	at(m, 2, 2).InstancesAdd(wall)
	at(m, 3, 3).InstancesAdd(wall)
	check := func(want int) {
		t.Helper()
		if got := smoothJunction(m, at(m, 2, 2).Coord, wall, rules); got != want {
			t.Fatalf("junction = %d, want %d", got, want)
		}
	}
	check(0) // A diagonal alone must not connect.
	at(m, 2, 3).InstancesAdd(wall)
	check(1)
	at(m, 3, 2).InstancesAdd(prefab("/obj/structure/window", map[string]string{"anchored": "0", "smoothing_groups": `list(-2)`}))
	check(1) // Unanchored movables do not connect.
	at(m, 3, 2).Set(nil)
	at(m, 3, 2).InstancesAdd(prefab("/obj/structure/window", map[string]string{"anchored": "1", "smoothing_groups": `list(-2)`, "canSmoothWith": `list(99)`}))
	check(21) // Groups are directional; the window need not accept the wall.
	wall = withVars(wall, map[string]string{"smoothing_flags": "2"})
	check(5)
	if got := smoothJunction(m, at(m, 1, 1).Coord, withVars(wall, map[string]string{"smoothing_flags": "9"}), rules); got != 74 {
		t.Fatalf("border junction = %d, want 74", got)
	}
}

func TestSmoothingUsesProjectFlags(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "code", "__DEFINES")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "icon_smoothing.dm"), []byte("#define SMOOTH_CORNERS (1<<0)\n#define SMOOTH_BITMASK (1<<1)\n#define SMOOTH_DIAGONAL (1<<2)\n#define SMOOTH_BORDER (1<<3)\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := loadRules(root)
	if r.bitmask != 2 || r.cardinals != 0 || r.diagonal != 4 || r.border != 8 {
		t.Fatalf("wrong legacy rules: %+v", r)
	}
}

func TestDiagonalUnderlayAndAreaBoundary(t *testing.T) {
	m := emptyMap(3, 3, 1)
	w := prefab("/turf/closed/wall", map[string]string{"smoothing_flags": "5", "base_icon_state": `"wall"`, "icon": `'wall.dmi'`})
	for _, pos := range [][2]int{{2, 2}, {2, 3}, {3, 2}} {
		at(m, pos[0], pos[1]).InstancesAdd(w)
	}
	floor := prefab("/turf/open/floor", map[string]string{"icon_state": `"iron"`})
	at(m, 2, 1).InstancesAdd(floor)
	scene := Build(m, &dmenv.Dme{}, Options{Smoothing: true}, func(_, _ string) bool { return true })
	center := at(scene.Map, 2, 2).Instances()
	if len(center) != 2 || center[0].Prefab().Vars().TextV("icon_state", "") != "wall-5-d" || center[1].Prefab().Path() != floor.Path() {
		t.Fatal("missing diagonal sprite or underlay")
	}
	if len(at(m, 2, 2).Instances()) != 1 {
		t.Fatal("added diagonal floor to source")
	}
	at(m, 2, 2).InstancesAdd(prefab("/area/limited", map[string]string{"area_limited_icon_smoothing": "/area/limited"}))
	at(m, 2, 3).InstancesAdd(prefab("/area/limited/child", nil))
	at(m, 3, 2).InstancesAdd(prefab("/area/other", nil))
	if got := smoothJunction(m, at(m, 2, 2).Coord, w, loadRules("")); got != 1 {
		t.Fatalf("smoothed across area boundary: %d", got)
	}
}

func TestBuildPreservesSourceAndMissingStates(t *testing.T) {
	m := emptyMap(2, 1, 2)
	w := prefab("/turf/closed/wall", map[string]string{"smoothing_flags": "1", "base_icon_state": `"wall"`, "icon_state": `"map"`, "icon": `'wall.dmi'`})
	for _, tile := range m.Tiles {
		tile.InstancesAdd(w)
	}
	snapshot := Snapshot(m, 2)
	if snapshot.MaxZ != 1 || snapshot.Tiles[0].Instances()[0].Coord().Z != 1 {
		t.Fatal("snapshot did not select one level")
	}
	scene := Build(snapshot, &dmenv.Dme{}, Options{Smoothing: true}, func(_, state string) bool { return state == "wall-4" })
	if scene.Smoothed != 1 || len(scene.Notes) != 1 {
		t.Fatalf("wrong scene: %+v", scene)
	}
	if got := scene.Map.Tiles[0].Instances()[0].Prefab().Vars().TextV("icon_state", ""); got != "wall-4" {
		t.Fatal(got)
	}
	for _, source := range []*dmmap.Dmm{m, snapshot} {
		for _, tile := range source.Tiles {
			if tile.Instances()[0].Prefab() != w {
				t.Fatal("changed source prefab")
			}
		}
	}
	plain := Build(snapshot, &dmenv.Dme{}, Options{}, func(_, _ string) bool { t.Fatal("smoothing called when disabled"); return false })
	if plain.Map.Tiles[0].Instances()[0].Prefab() != w || plain.Lighting != nil {
		t.Fatal("options did not disable effects")
	}
}

func TestLightFalloffAndCone(t *testing.T) {
	l := lightSource{radius: 8, height: 1, angle: 360}
	if got := l.falloff(0, 0); got != .875 {
		t.Fatal(got)
	}
	if got := l.falloff(3, 4); math.Abs(got-(1-math.Sqrt(26)/8)) > 1e-8 {
		t.Fatal(got)
	}
	if l.falloff(8, 0) != 0 {
		t.Fatal("light extends beyond radius")
	}
	l.angle, l.dir = 90, 1
	if l.falloff(0, -1) != 0 || l.falloff(0, 1) == 0 {
		t.Fatal("cone direction incorrect")
	}
}

func TestOpaqueWallBlocksColoredLight(t *testing.T) {
	m := emptyMap(9, 7, 1)
	at(m, 2, 4).InstancesAdd(prefab("/obj/light", map[string]string{"light_range": "12", "light_color": `"#ff0000"`}))
	for y := 1; y <= 7; y++ {
		at(m, 5, y).InstancesAdd(prefab("/turf/closed/wall", map[string]string{"opacity": "1"}))
	}
	l, n := buildLighting(m, true, false)
	if n != 1 || l.Tile(1, 3)[0][0] <= 0 || l.Tile(1, 3)[0][1] != 0 {
		t.Fatal("colored light missing")
	}
	for y := 0; y < 7; y++ {
		for x := 5; x < 9; x++ {
			if got := l.Tile(x, y); got != [4]RGB{} {
				t.Fatalf("light leaked behind wall at %d,%d: %v", x, y, got)
			}
		}
	}
	if l.Tile(4, 3)[0][0] == 0 {
		t.Fatal("visible wall face is not illuminated")
	}
	opaque := []bool{false, true, true, false}
	if lineVisible(opaque, 2, 2, 0, 0, 1, 1) {
		t.Fatal("light leaked between diagonal walls")
	}
}

func TestFixturesAndAreaLighting(t *testing.T) {
	m := emptyMap(2, 1, 1)
	fixture := prefab("/obj/machinery/light", map[string]string{"on": "0", "brightness": "8", "bulb_power": "2", "bulb_colour": `"#ff8000"`})
	at(m, 1, 1).InstancesAdd(fixture)
	at(m, 2, 1).InstancesAdd(withVars(fixture, map[string]string{"status": "2"}))
	l, n := buildLighting(m, true, false)
	if n != 1 {
		t.Fatalf("expected powered fixture but not broken fixture: %d", n)
	}
	color := l.Tile(0, 0)[0]
	if color[0] != 1 || color[1] < .49 || color[1] > .52 || color[2] != 0 {
		t.Fatalf("wrong normalized light: %v", color)
	}
	l, n = buildLighting(m, false, false)
	if n != 0 || l.Tile(0, 0) != [4]RGB{} {
		t.Fatal("powered fixture toggle ignored")
	}
	at(m, 1, 1).InstancesAdd(prefab("/area/lit", map[string]string{"static_lighting": "0"}))
	at(m, 2, 1).InstancesAdd(prefab("/area/ambient", map[string]string{"base_lighting_alpha": "255", "base_lighting_color": `"#0000ff"`}))
	l, _ = buildLighting(m, false, false)
	if !reflect.DeepEqual(l.Tile(0, 0), [4]RGB{{1, 1, 1}, {1, 1, 1}, {1, 1, 1}, {1, 1, 1}}) || l.Tile(1, 0)[0] != (RGB{0, 0, 1}) {
		t.Fatal("area lighting crossed area boundaries")
	}
}

func TestCompiledAppearancesPreservesStagedOverrides(t *testing.T) {
	initial := prefab("/turf/open/test", map[string]string{"icon_state": `"editor"`, "dir": "2"})
	compiled := prefab(initial.Path(), map[string]string{"icon_state": `"game"`, "dir": "2"})
	editorEnv := &dmenv.Dme{Objects: map[string]*dmenv.Object{initial.Path(): {Path: initial.Path(), Vars: initial.Vars()}}}
	gameEnv := &dmenv.Dme{Objects: map[string]*dmenv.Object{initial.Path(): {Path: initial.Path(), Vars: compiled.Vars()}}}
	m := emptyMap(2, 1, 1)
	at(m, 1, 1).InstancesAdd(withVars(initial, map[string]string{"dir": "4"}))
	staged := withVars(withVars(initial, map[string]string{"dir": "8"}), map[string]string{"icon_state": `"custom"`})
	at(m, 2, 1).InstancesAdd(staged)
	result := CompiledAppearances(m, editorEnv, gameEnv)
	first := result.Tiles[0].Instances()[0].Prefab().Vars()
	second := result.Tiles[1].Instances()[0].Prefab().Vars()
	if first.TextV("icon_state", "") != "game" || first.IntV("dir", 0) != 4 || second.TextV("icon_state", "") != "custom" || second.IntV("dir", 0) != 8 {
		t.Fatal("compiled appearance dropped overrides")
	}
	if at(m, 1, 1).Instances()[0].Prefab().Vars().TextV("icon_state", "") != "editor" || at(m, 2, 1).Instances()[0].Prefab() != staged {
		t.Fatal("replaced source definitions")
	}
}
