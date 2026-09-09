package mappreview

import (
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
)

func TestWindowSpawnersConnectShuttleWalls(t *testing.T) {
	wall := prefab("/turf/closed/wall/mineral/titanium", map[string]string{
		"smoothing_flags": "5", "base_icon_state": `"shuttle_wall"`, "icon": `'wall.dmi'`,
		"smoothing_groups": `"40,"`, "canSmoothWith": `"-67,40,"`,
	})
	window := prefab("/obj/structure/window/reinforced/shuttle", map[string]string{
		"anchored": "1", "smoothing_flags": "1", "base_icon_state": `"shuttle_window"`, "icon": `'window.dmi'`,
		"smoothing_groups": `"-67,-68,"`, "canSmoothWith": `"-68,"`,
	})
	grille := prefab("/obj/structure/grille", nil)
	spawner := prefab("/obj/effect/spawner/structure/window/reinforced/shuttle", map[string]string{
		"spawn_list": "list(/obj/structure/grille, /obj/structure/window/reinforced/shuttle)",
	})
	dme := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	for _, p := range []*dmmprefab.Prefab{window, grille} {
		dme.Objects[p.Path()] = &dmenv.Object{Path: p.Path(), Vars: p.Vars()}
	}
	m := emptyMap(3, 3, 1)
	at(m, 2, 2).InstancesAdd(wall)
	at(m, 2, 3).InstancesAdd(spawner)
	at(m, 3, 2).InstancesAdd(spawner)
	at(m, 3, 1).InstancesAdd(spawner)
	scene := Build(m, dme, Options{Smoothing: true}, func(_, _ string) bool { return true })
	if got := at(scene.Map, 2, 2).Instances()[0].Prefab().Vars().TextV("icon_state", ""); got != "shuttle_wall-5-d" {
		t.Fatalf("wall did not connect to spawned windows: %s", got)
	}
	glass := at(scene.Map, 3, 2).Instances()
	if len(glass) != 2 || glass[0].Prefab().Path() != grille.Path() || glass[1].Prefab().Vars().TextV("icon_state", "") != "shuttle_window-2" {
		t.Fatal("spawner did not produce a grille and connected window")
	}
	if len(scene.Notes) != 0 || at(m, 2, 2).Instances()[0].Prefab() != wall || at(m, 3, 2).Instances()[0].Prefab() != spawner {
		t.Fatal("preview changed the source or reported unsupported smoothing")
	}
}

func TestAirlockFillsAndDoorLayers(t *testing.T) {
	for _, tc := range []struct {
		name, density, glass, material, icon, state string
	}{
		{"solid", "1", "0", "null", "door.dmi", "fill_closed"},
		{"open", "0", "0", "null", "door.dmi", "fill_open"},
		{"glass", "1", "1", "null", "overlays.dmi", "glass_closed"},
		{"material", "1", "0", `"gold"`, "overlays.dmi", "gold_closed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := emptyMap(1, 1, 1)
			fan := prefab("/obj/structure/fans/tiny", map[string]string{"layer": "2"})
			door := prefab("/obj/machinery/door/airlock", map[string]string{
				"icon": `'door.dmi'`, "icon_state": `"closed"`, "overlays_file": `'overlays.dmi'`,
				"density": tc.density, "glass": tc.glass, "airlock_material": tc.material,
				"layer": "1", "closingLayer": "3", "dir": "4", "pixel_x": "2", "plane": "-1",
			})
			at(m, 1, 1).InstancesAdd(fan)
			at(m, 1, 1).InstancesAdd(door)
			scene := Build(m, &dmenv.Dme{}, Options{}, func(_, _ string) bool { return true })
			items := scene.Map.Tiles[0].Instances()
			if len(items) != 3 {
				t.Fatalf("door fill missing: %d appearances", len(items))
			}
			base, fill := items[1].Prefab().Vars(), items[2].Prefab().Vars()
			if fill.TextV("icon", "") != tc.icon || fill.TextV("icon_state", "") != tc.state ||
				fill.IntV("dir", 0) != 4 || fill.IntV("pixel_x", 0) != 2 || fill.IntV("plane", 0) != -1 {
				t.Fatal("wrong fill appearance or position")
			}
			if fill.FloatV("layer", 0) <= base.FloatV("layer", 0) {
				t.Fatal("fill is behind its frame")
			}
			if tc.density == "1" && base.FloatV("layer", 0) <= fan.Vars().FloatV("layer", 0) {
				t.Fatal("closed door is below the fan")
			}
			if tc.density == "0" && (base.FloatV("layer", 0) != 1 || base.TextV("icon_state", "") != "open") {
				t.Fatal("open door lost its open sprite or layer")
			}
			if len(at(m, 1, 1).Instances()) != 2 || at(m, 1, 1).Instances()[1].Prefab() != door {
				t.Fatal("preview changed the source door")
			}
		})
	}
}

func TestCoveredUtilitiesAreHidden(t *testing.T) {
	for _, access := range []string{"0", "1", "2"} {
		m := emptyMap(1, 1, 1)
		for _, p := range []*dmmprefab.Prefab{
			prefab("/turf/open/floor", map[string]string{"underfloor_accessibility": access}),
			prefab("/obj/structure/cable", nil),
			prefab("/obj/machinery/atmospherics/pipe/simple", map[string]string{"hide": "1"}),
			prefab("/obj/machinery/atmospherics/pipe/simple/visible", map[string]string{"hide": "0"}),
			prefab("/obj/machinery/atmospherics/components/unary/vent_pump", map[string]string{"hide": "1"}),
			prefab("/obj/structure/fans/tiny", nil),
		} {
			at(m, 1, 1).InstancesAdd(p)
		}
		scene := Build(m, &dmenv.Dme{}, Options{}, nil)
		want := 6
		if access == "0" {
			want = 4
		}
		if len(scene.Map.Tiles[0].Instances()) != want || len(m.Tiles[0].Instances()) != 6 {
			t.Fatalf("incorrect underfloor visibility for accessibility %s", access)
		}
	}
}
