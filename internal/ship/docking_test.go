package ship

import (
	"bytes"
	"testing"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/util"
)

func TestDockingPortPlacement(t *testing.T) {
	for _, size := range []struct{ w, h, preferred int }{{20, 16, 1}, {16, 20, 4}, {20, 20, 1}} {
		c, env := authorEnvironment(t)
		p, err := NewProject(c, env, "dock_fixture", "Dock fixture", size.w, size.h)
		if err != nil {
			t.Fatal(err)
		}
		theme := p.Hull.Themes[0]
		file, _ := c.HullFile(p.Hull, theme)
		m := p.Documents[file].Map
		if err = p.Deck(theme, util.Point{X: 4, Y: 4, Z: 1}, util.Point{X: 12, Y: 10, Z: 1}); err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct{ x, y, out, in, relative int }{{8, 10, 1, 2, 1}, {12, 7, 4, 8, 4}, {8, 4, 2, 1, 2}, {4, 7, 8, 4, 8}} {
			point := util.Point{X: test.x, Y: test.y, Z: 1}
			m.GetTile(point).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/machinery/door/airlock"))
			m.GetTile(point).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
			before := p.Capture()
			beforeBytes := RawData(m).EncodeTGM()
			if err = p.PlaceDockingPort(theme, point, point, test.out); err != nil {
				t.Fatal(err)
			}
			site, err := p.InspectDockingSite(theme, point, point)
			if err != nil {
				t.Fatal(err)
			}
			if !site.Existing || !site.Airlock || site.CurrentOutward != test.out {
				t.Fatalf("wrong site: %+v", site)
			}
			for key, value := range map[string]int{"dir": test.in, "port_direction": test.relative, "preferred_direction": size.preferred} {
				if got := portDirection(site.port, key, 0); got != value {
					t.Fatalf("%s: got %d, want %d", key, got, value)
				}
			}
			for _, tile := range before.Maps[file].Tiles {
				old := dmmdata.Prefabs{}
				now := dmmdata.Prefabs{}
				for _, inst := range tile.Instances() {
					if !childPath(inst.Prefab().Path(), "/obj/docking_port/mobile") {
						old = append(old, inst.Prefab())
					}
				}
				for _, inst := range m.GetTile(tile.Coord).Instances() {
					if !childPath(inst.Prefab().Path(), "/obj/docking_port/mobile") {
						now = append(now, inst.Prefab())
					}
				}
				if len(old) != len(now) {
					t.Fatal("port placement changed other tile contents")
				}
				for i := range old {
					if old[i].Id() != now[i].Id() {
						t.Fatal("port placement changed unrelated prefab")
					}
				}
			}
			after := p.Capture()
			p.Restore(before)
			if !bytes.Equal(beforeBytes, RawData(m).EncodeTGM()) {
				t.Fatal("undo did not restore the map")
			}
			p.Restore(after)
		}
		if err = p.Save(); err != nil {
			t.Fatal(err)
		}
		reopened, err := OpenProject(c, env, p.Hull)
		if err != nil {
			t.Fatal(err)
		}
		point := util.Point{X: 4, Y: 7, Z: 1}
		site, err := reopened.InspectDockingSite(theme, point, point)
		if err != nil || site.CurrentOutward != 8 || portDirection(site.port, "preferred_direction", 0) != size.preferred {
			t.Fatalf("saved port did not round-trip: %v", err)
		}
	}
}

func TestDockingPortSelectionAndValidation(t *testing.T) {
	c, env := authorEnvironment(t)
	p, err := NewProject(c, env, "entrance_fixture", "Entrance fixture", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	theme := p.Hull.Themes[0]
	file, _ := c.HullFile(p.Hull, theme)
	m := p.Documents[file].Map
	lo, hi := util.Point{X: 4, Y: 4, Z: 1}, util.Point{X: 12, Y: 10, Z: 1}
	if err = p.Deck(theme, lo, hi); err != nil {
		t.Fatal(err)
	}
	door := util.Point{X: 8, Y: 4, Z: 1}
	m.GetTile(door).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/machinery/door/airlock"))
	site, err := p.InspectDockingSite(theme, lo, hi)
	if err != nil || site.Position != door {
		t.Fatalf("did not find the one selected airlock: %v", err)
	}
	before := RawData(m).EncodeTGM()
	for _, point := range []util.Point{{X: 8, Y: 7, Z: 1}, {X: 2, Y: 2, Z: 1}, {X: 0, Y: 0, Z: 1}} {
		if err = p.PlaceDockingPort(theme, point, point, 2); err == nil {
			t.Fatal("accepted interior, unowned or invalid tile")
		}
	}
	if err = p.PlaceDockingPort(theme, door, door, 1); err == nil {
		t.Fatal("accepted facing into the ship")
	}
	if !bytes.Equal(before, RawData(m).EncodeTGM()) {
		t.Fatal("invalid placement changed the map")
	}
	m.GetTile(lo).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/machinery/door/airlock"))
	if _, err = p.InspectDockingSite(theme, lo, hi); err == nil {
		t.Fatal("accepted two airlocks")
	}
	// New authored ships can recreate a deleted port without inventing a type.
	for _, tile := range m.Tiles {
		tile.InstancesRemoveByPath("/obj/docking_port/mobile")
	}
	if err = p.PlaceDockingPort(theme, door, door, 2); err != nil {
		t.Fatal(err)
	}
	site, err = p.InspectDockingSite(theme, door, door)
	if err != nil || !site.Existing || site.port.Path() != p.portType() {
		t.Fatalf("failed to create matching port: %v", err)
	}
	// Hand-authored ships keep their actual type and unrelated per-instance vars.
	portType := site.port.Path()
	p.Settings = nil
	m.GetTile(door).InstancesRemoveByPath("/obj/docking_port/mobile")
	m.GetTile(door).InstancesAdd(p.prefab(portType, map[string]string{"name": `"Custom port"`, "dir": "4", "port_direction": "1", "shuttle_id": `"preserve_me"`}))
	if err = p.PlaceDockingPort(theme, door, door, 2); err != nil {
		t.Fatal(err)
	}
	site, err = p.InspectDockingSite(theme, door, door)
	if err != nil || site.port.Path() != portType || site.port.Vars().ValueV("shuttle_id", "") != `"preserve_me"` || portDirection(site.port, "port_direction", 0) != 8 {
		t.Fatalf("lost legacy identity or relative facing: %v", err)
	}
	m.GetTile(lo).InstancesAdd(site.port)
	before = RawData(m).EncodeTGM()
	if err = p.PlaceDockingPort(theme, door, door, 2); err == nil {
		t.Fatal("accepted duplicate mobile ports")
	}
	if !bytes.Equal(before, RawData(m).EncodeTGM()) {
		t.Fatal("duplicate rejection changed the map")
	}
}
