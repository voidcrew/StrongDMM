package ship

import (
	"sdmm/internal/util"
	"testing"
)

func TestCreationRejectsDuplicateDisplayNames(t *testing.T) {
	c, d := authorEnvironment(t)
	p, err := NewProject(c, d, "one", "Little Explorer", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	c.Hulls = append(c.Hulls, p.Hull)
	for _, name := range []string{"Little Explorer", " little explorer ", "LITTLE   EXPLORER"} {
		if _, err = NewProject(c, d, "another", name, 20, 20); err == nil {
			t.Fatalf("accepted duplicate ship %q", name)
		}
	}
	if err = ShipNameError(c, d, " LITTLE Explorer ", p.Hull.Type); err != nil {
		t.Fatal("ship cannot retain its own name", err)
	}
	lo, hi := util.Point{X: 2, Y: 2, Z: 1}, util.Point{X: 4, Y: 4, Z: 1}
	if err = p.addRect(0, "cargo", "Cargo Bay", lo, hi); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Hull", " empty ROOM "} {
		if err = p.ModuleNameError(name); err == nil {
			t.Fatal("accepted a name reserved by the part/room dropdown", name)
		}
	}
	for _, name := range []string{"Cargo Bay", " cargo bay ", "CARGO   BAY"} {
		if err = p.addRect(0, "another_room", name, util.Point{X: 8, Y: 8, Z: 1}, util.Point{X: 10, Y: 10, Z: 1}); err == nil {
			t.Fatalf("accepted duplicate room %q", name)
		}
		if err = p.AddModule(0, p.Hull.Modules[0], "another_option", name, true); err == nil {
			t.Fatalf("accepted duplicate option %q", name)
		}
	}
	if err = p.AddTheme(0, "salvager", "Salvager", true); err != nil {
		t.Fatal(err)
	}
	if err = p.AddTheme(0, "salvager_two", " SALVAGER ", true); err == nil {
		t.Fatal("accepted duplicate theme")
	}
	if _, err = p.AddArea(p.Hull.Themes[0], "bridge", "Main Bridge", "bridge"); err != nil {
		t.Fatal(err)
	}
	if _, err = p.AddArea(p.Hull.Themes[1], "bridge_two", " main   BRIDGE ", "bridge"); err == nil {
		t.Fatal("accepted duplicate area across themes")
	}
	if len(p.Hull.Modules) != 1 || len(p.Hull.Themes) != 2 || len(p.RoomAreas) != 1 {
		t.Fatal("rejected names changed registrations")
	}
	other, err := NewProject(c, d, "second", "Another Ship", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	c.Hulls = append(c.Hulls, other.Hull)
	other.Hull.Name = "little explorer"
	if err = other.Save(); err == nil {
		t.Fatal("save accepted duplicate renamed ship")
	}
}
