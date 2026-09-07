package ship

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

func prefab(path string, values ...string) *dmmprefab.Prefab {
	v := &dmvars.MutableVariables{}
	for i := 0; i < len(values); i += 2 {
		v.Put(values[i], values[i+1])
	}
	return dmmprefab.New(0, path, v.ToImmutable())
}

func fixture(width, height int) *dmmdata.DmmData {
	d := &dmmdata.DmmData{MaxX: width, MaxY: height, MaxZ: 1, Dictionary: dmmdata.DataDictionary{}, Grid: dmmdata.DataGrid{}}
	for y := 1; y <= height; y++ {
		for x := 1; x <= width; x++ {
			p := util.Point{X: x, Y: y, Z: 1}
			key := dmmdata.Key(p.String())
			d.Grid[p] = key
			d.Dictionary[key] = dmmdata.Prefabs{prefab("/turf/template_noop"), prefab("/area/template_noop")}
		}
	}
	return d
}

func put(d *dmmdata.DmmData, x, y int, values ...*dmmprefab.Prefab) {
	d.Dictionary[d.Grid[util.Point{X: x, Y: y, Z: 1}]] = values
}

func TestCompositionPreservesMixedOwnershipAndCoordinates(t *testing.T) {
	hull := fixture(8, 8)
	put(hull, 5, 6, prefab(SlotMarker, "key", `"med"`), prefab("/obj/machinery/vent"), prefab("/turf/open/floor/hull"), prefab("/area/ship"))
	module := fixture(3, 4)
	put(module, 2, 3, prefab(Connector), prefab("/obj/structure/table"), prefab("/turf/template_noop"), prefab("/area/template_noop"))
	put(module, 3, 3, prefab("/turf/open/floor/module"), prefab("/area/template_noop"))
	a, err := Compose([]Source{{Name: "Hull", Data: hull}, {Name: "Medical", Slot: "med", Data: module}})
	if err != nil {
		t.Fatal(err)
	}
	if a.Sources[1].Offset != (util.Point{X: 3, Y: 3}) {
		t.Fatal(a.Sources[1].Offset)
	}
	atoms := a.Cells[util.Point{X: 5, Y: 6, Z: 1}]
	if len(atoms) != 4 {
		t.Fatalf("expected hull vent, floor, area and module table: %#v", atoms)
	}
	for _, atom := range atoms {
		if atom.Prefab.Path() == "/obj/structure/table" {
			if atom.Source != 1 || atom.Local != (util.Point{X: 2, Y: 3, Z: 1}) {
				t.Fatal("module source coordinates lost")
			}
		} else if atom.Source != 0 {
			t.Fatal("hull ownership lost")
		}
	}
	if len(module.Dictionary[module.Grid[util.Point{X: 2, Y: 3, Z: 1}]]) != 4 {
		t.Fatal("source was mutated")
	}
	if got := a.Cells[util.Point{X: 6, Y: 6, Z: 1}]; len(got) != 1 || got[0].Prefab.Path() != "/turf/open/floor/module" {
		t.Fatal("connector offset was not applied")
	}
}

func TestRejectInvalidPlacement(t *testing.T) {
	for _, scenario := range []string{"missing connector", "duplicate connector", "duplicate marker", "outside hull", "overlap"} {
		t.Run(scenario, func(t *testing.T) {
			hull := fixture(2, 2)
			put(hull, 2, 2, prefab(SlotMarker, "key", `"slot"`))
			module := fixture(2, 1)
			put(module, 1, 1, prefab(Connector), prefab("/turf/open/floor/test"))
			sources := []Source{{Name: "Hull", Data: hull}, {Name: "Module", Slot: "slot", Data: module}}
			switch scenario {
			case "missing connector":
				put(module, 1, 1, prefab("/turf/open/floor/test"))
			case "duplicate connector":
				put(module, 2, 1, prefab(Connector))
			case "duplicate marker":
				put(hull, 1, 1, prefab(SlotMarker, "key", `"slot"`))
			case "outside hull":
				put(module, 2, 1, prefab("/obj/structure/table"))
			case "overlap":
				put(hull, 2, 2, prefab(SlotMarker, "key", `"slot"`), prefab(SlotMarker, "key", `"other"`))
				sources = append(sources, Source{Name: "Other", Slot: "other", Data: module})
			}
			if _, err := Compose(sources); err == nil {
				t.Fatal("invalid placement accepted")
			}
		})
	}
}

func TestTurfReplacementDoesNotRemoveHullObjects(t *testing.T) {
	hull := fixture(1, 1)
	put(hull, 1, 1, prefab(SlotMarker, "key", `"slot"`), prefab("/obj/machinery/apc"), prefab("/turf/open/floor/old"), prefab("/area/ship"))
	m := fixture(1, 1)
	put(m, 1, 1, prefab(Connector), prefab("/turf/open/floor/new"), prefab("/area/template_noop"))
	a, err := Compose([]Source{{Data: hull}, {Data: m, Slot: "slot"}})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, atom := range a.Cells[util.Point{X: 1, Y: 1, Z: 1}] {
		paths = append(paths, atom.Prefab.Path())
	}
	if !reflect.DeepEqual(paths, []string{"/obj/machinery/apc", "/area/ship", "/turf/open/floor/new"}) {
		t.Fatal(paths)
	}
}

func TestThemeResolutionAndPathContainment(t *testing.T) {
	root := t.TempDir()
	c := Catalog{Root: root, ModuleDir: "modules"}
	if err := os.Mkdir(filepath.Join(root, "modules"), 0700); err != nil {
		t.Fatal(err)
	}
	m := Module{ID: "test", Name: "Test", File: "med.dmm"}
	themed := filepath.Join(root, "modules", "med_blue.dmm")
	if err := os.WriteFile(themed, []byte("themed only"), 0600); err != nil {
		t.Fatal(err)
	}
	if file, err := c.ModuleFile(m, "blue"); err != nil || file != themed {
		t.Fatalf("themed-only source rejected: %s %v", file, err)
	}
	if _, err := c.ModuleFile(m, "red"); err == nil {
		t.Fatal("missing variant accepted")
	}
	base := filepath.Join(root, "modules", "med.dmm")
	if err := os.WriteFile(base, []byte("base"), 0600); err != nil {
		t.Fatal(err)
	}
	if file, err := c.ModuleFile(m, "red"); err != nil || file != base {
		t.Fatal("base fallback missing")
	}
	for _, p := range []string{"../escape.dmm", filepath.Join(root, "absolute.dmm")} {
		if _, err := Inside(root, p); err == nil {
			t.Fatal("path escaped project", p)
		}
	}
}

func TestStringLists(t *testing.T) {
	for _, input := range []string{`list("one", "two",)`, `list("one", "two")`} {
		got, err := stringList(input)
		if err != nil || !reflect.DeepEqual(got, []string{"one", "two"}) {
			t.Fatal(got, err)
		}
	}
	if list, err := stringList("list()"); err != nil || list == nil {
		t.Fatal("empty override must differ from null")
	}
	for _, input := range []string{`list("one" = 2)`, `list(/datum/foo)`, `list("broken)`} {
		if _, err := stringList(input); err == nil {
			t.Fatal("accepted", input)
		}
	}
}

func TestBlockedCryopod(t *testing.T) {
	a := &Assembly{Cells: map[util.Point][]Atom{}}
	pod := util.Point{X: 2, Y: 2, Z: 1}
	exit := util.Point{X: 2, Y: 1, Z: 1}
	a.Cells[pod] = []Atom{{Prefab: prefab("/obj/machinery/cryopod")}}
	a.Cells[exit] = []Atom{{Prefab: prefab("/turf/open/floor/iron")}, {Prefab: prefab("/obj/structure/table", "density", "1")}}
	dme := &dmenv.Dme{Objects: map[string]*dmenv.Object{}}
	a.CheckAccess(dme)
	if len(a.Issues) != 1 || !strings.Contains(a.Issues[0].Message, "Cryopod") {
		t.Fatal(a.Issues)
	}
	a.Issues = nil
	a.Cells[exit] = a.Cells[exit][:1]
	a.CheckAccess(dme)
	if len(a.Issues) != 0 {
		t.Fatal("clear exit flagged", a.Issues)
	}
}
