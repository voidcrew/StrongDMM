package ship

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"
)

func TestRoomAreasPreserveMapsAndRoundTrip(t *testing.T) {
	c, d := authorEnvironment(t)
	p, err := NewProject(c, d, "room_test", "Room Test", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	theme := p.Hull.Themes[0]
	file, _ := c.HullFile(p.Hull, theme)
	lo, hi := util.Point{X: 4, Y: 5, Z: 1}, util.Point{X: 7, Y: 8, Z: 1}
	if err = p.Deck(theme, lo, hi); err != nil {
		t.Fatal(err)
	}
	tile := p.Documents[file].Map.GetTile(lo)
	tile.InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
	tile.InstancesAdd(dmmap.PrefabStorage.Initial("/obj/machinery/power/apc"))
	nonAreas := func() string {
		var b strings.Builder
		for _, inst := range tile.Instances() {
			if !strings.HasPrefix(inst.Prefab().Path(), "/area/") {
				fmt.Fprintf(&b, "%d:%s;", inst.Id(), inst.Prefab().Path())
			}
		}
		return b.String()
	}
	beforeContent := nonAreas()
	before := p.Capture()
	bridge, err := p.AddArea(theme, "bridge", "Bridge [A]", "bridge")
	if err != nil {
		t.Fatal(err)
	}
	cargo, err := p.AddArea(theme, "cargo", "Cargo", "quart")
	if err != nil {
		t.Fatal(err)
	}
	if err = p.AssignArea(theme, file, bridge, lo, lo); err != nil {
		t.Fatal(err)
	}
	if err = p.AssignArea(theme, file, cargo, hi, hi); err != nil {
		t.Fatal(err)
	}
	if beforeContent != nonAreas() {
		t.Fatal("assigning an area changed turf or objects")
	}
	if d.Objects[bridge].Parent().Path != p.areaType() || d.Objects[cargo].Parent().Path != p.areaType() {
		t.Fatal("room areas do not inherit the ship area")
	}
	if _, err = p.AddArea(theme, "cargo", "Duplicate", "station"); err == nil {
		t.Fatal("duplicate area accepted")
	}
	if err = p.AssignArea(theme, file, "/area/space", lo, hi); err == nil {
		t.Fatal("unrelated area accepted")
	}
	after := p.Capture()
	p.Restore(before)
	areas, err := p.Areas(theme)
	if err != nil || len(areas) != 1 {
		t.Fatalf("undo kept new areas in the list: %v %+v", err, areas)
	}
	p.Restore(after)
	if err = p.AddSlot(0, "cargo", "Cargo", lo, hi); err != nil {
		t.Fatal(err)
	}
	a, err := p.Assemble(p.Hull.Themes[0], map[string]string{"cargo": "cargo_basic"})
	if err != nil {
		t.Fatal(err)
	}
	hullBefore := RawData(a.Sources[0].Live).EncodeTGM()
	local := util.Point{X: 2, Y: 2, Z: 1}
	if err = p.AssignArea(p.Hull.Themes[0], a.Sources[1].File, cargo, local, local); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(hullBefore, RawData(a.Sources[0].Live).EncodeTGM()) {
		t.Fatal("assigning a module area changed hull")
	}
	if err = p.AddTheme(0, "salvager", "Salvager"); err != nil {
		t.Fatal(err)
	}
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	_, code, _ := p.areaPaths()
	b, err := os.ReadFile(code)
	if err != nil || !strings.Contains(string(b), `Bridge \[A]`) || !strings.Contains(string(b), cargo) {
		t.Fatalf("bad generated area definitions: %v %s", err, b)
	}
	include, _ := os.ReadFile(d.RootFile)
	if bytes.Count(include, []byte("#include")) != 3 {
		t.Fatalf("area registration missing or duplicated: %s", include)
	}
	reopened, err := OpenProject(c, d, p.Hull)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.RoomAreas) != 2 || reopened.Modified() {
		t.Fatal("area definitions failed to reopen cleanly")
	}
	for _, variant := range reopened.Hull.Themes {
		a, err = reopened.Assemble(variant, map[string]string{"cargo": "cargo_basic"})
		if err != nil {
			t.Fatal(err)
		}
		global := util.Point{X: lo.X + 1, Y: lo.Y + 1, Z: 1}
		found := false
		for _, atom := range a.Cells[global] {
			if atom.Prefab.Path() == cargo && atom.Source == 1 {
				found = true
			}
		}
		if !found {
			t.Fatal("theme lost module area override")
		}
	}
	if err = reopened.Save(); err != nil {
		t.Fatal(err)
	}
	includeAgain, _ := os.ReadFile(d.RootFile)
	if !bytes.Equal(include, includeAgain) {
		t.Fatal("no-op save rewrote includes")
	}
}

func TestAddAreasToExistingShipPreservesHandwrittenRegistration(t *testing.T) {
	c, d := authorEnvironment(t)
	p, err := NewProject(c, d, "legacy_ship", "Legacy", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	paths := p.outputPaths()
	registration, _ := os.ReadFile(paths[1])
	if err = os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	} // model an ordinary hand-authored ship
	legacy, err := OpenProject(c, d, p.Hull)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Settings != nil || legacy.Modified() {
		t.Fatal("existing ship was not opened cleanly")
	}
	area, err := legacy.AddArea(p.Hull.Themes[0], "new_lab", "New lab", "medbay")
	if err != nil {
		t.Fatal(err)
	}
	if area != "/area/shuttle/voidcrew/legacy_ship/new_lab" {
		t.Fatal("did not use docking port's ship area")
	}
	if err = legacy.Save(); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(paths[1])
	if !bytes.Equal(registration, after) {
		t.Fatal("area tool rewrote handwritten ship registration")
	}
	reopened, err := OpenProject(c, d, p.Hull)
	if err != nil || len(reopened.RoomAreas) != 1 || reopened.Modified() {
		t.Fatalf("existing ship areas did not reopen cleanly: %v", err)
	}
	_, code, _ := reopened.areaPaths()
	if err = os.WriteFile(code, []byte("external edit"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = reopened.AddArea(p.Hull.Themes[0], "second_lab", "Second lab", "medbay"); err != nil {
		t.Fatal(err)
	}
	if err = reopened.Save(); err == nil {
		t.Fatal("overwrote external source changes")
	}
}
