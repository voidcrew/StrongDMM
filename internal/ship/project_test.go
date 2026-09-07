package ship

import (
	"bytes"
	"os"
	"path/filepath"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
	"strings"
	"testing"
)

func authorEnvironment(t *testing.T) (*Catalog, *dmenv.Dme) {
	t.Helper()
	root := t.TempDir()
	d := &dmenv.Dme{RootDir: root, RootFile: filepath.Join(root, "test.dme"), Objects: map[string]*dmenv.Object{}}
	paths := []string{"/world", "/area", "/area/space", "/area/template_noop", "/area/shuttle/voidcrew", "/turf", "/turf/open/space", "/turf/template_noop", "/turf/open/floor/plating", "/obj/docking_port/mobile/voidcrew", SlotMarker, Connector, "/obj/item/test", "/obj/machinery/power/apc", HullType}
	for _, path := range paths {
		v := dmvars.MutableVariables{}
		if path == "/world" {
			v.Put("area", "/area/space")
			v.Put("turf", "/turf/open/space")
			v.Put("icon_size", "32")
		}
		d.Objects[path] = &dmenv.Object{Path: path, Vars: v.ToImmutable()}
	}
	dmmap.PrefabStorage.Free()
	dmicon.Cache.Free()
	dmmap.Init(d)
	if err := os.WriteFile(d.RootFile, []byte("// BEGIN_INCLUDE\n// END_INCLUDE\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return &Catalog{Root: root, ModuleDir: "_maps/voidcrew/ship_modules/"}, d
}
func TestNewShipAuthoringRoundTrip(t *testing.T) {
	c, d := authorEnvironment(t)
	p, err := NewProject(c, d, "fresh_ship", "Fresh [Ship]", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	theme := p.Hull.Themes[0]
	file, _ := c.HullFile(p.Hull, theme)
	hull := p.Documents[file].Map
	if _, err = os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("creation wrote map before Save")
	}
	lo, hi := util.Point{X: 4, Y: 5, Z: 1}, util.Point{X: 7, Y: 8, Z: 1}
	if err = p.Deck(theme, lo, hi); err != nil {
		t.Fatal(err)
	}
	hull.GetTile(lo).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
	hull.GetTile(lo).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/machinery/power/apc"))
	before := p.Capture()
	if err = p.AddSlot(0, "cargo", "Cargo", lo, hi); err != nil {
		t.Fatal(err)
	}
	module := p.Hull.Modules[0]
	selection := map[string]string{"cargo": module.ID}
	a, err := p.Assemble(p.Hull.Themes[0], selection)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Sources) != 2 || a.Sources[1].Offset.X != 3 || a.Sources[1].Offset.Y != 4 {
		t.Fatalf("wrong placement: %+v", a.Sources)
	}
	moduleMap := a.Sources[1].Live
	foundItem, foundAPC := false, false
	for _, atom := range a.Cells[lo] {
		if atom.Prefab.Path() == "/obj/item/test" {
			foundItem = true
			if atom.Source != 1 {
				t.Fatal("item was not extracted")
			}
		}
		if atom.Prefab.Path() == "/obj/machinery/power/apc" {
			foundAPC = true
			if atom.Source != 0 {
				t.Fatal("permanent APC moved into module")
			}
		}
	}
	if !foundItem || !foundAPC {
		t.Fatal("extraction lost content")
	}
	after := p.Capture()
	p.Restore(before)
	if len(p.Hull.Modules) != 0 {
		t.Fatal("undo kept registration")
	}
	p.Restore(after)
	if p.Documents[a.Sources[1].File].Map != moduleMap {
		t.Fatal("undo changed document identity")
	}
	if err = p.AddModule(0, module, "cargo_empty", "Empty", true); err != nil {
		t.Fatal(err)
	}
	if err = p.AddTheme(0, "pirate", "Pirate"); err != nil {
		t.Fatal(err)
	}
	pirate, err := p.Assemble(p.Hull.Themes[1], selection)
	if err != nil {
		t.Fatal(err)
	}
	if pirate.Sources[1].Live == moduleMap {
		t.Fatal("theme shares writable map")
	}
	if pirate.Sources[1].Live.GetTile(util.Point{X: 1, Y: 1, Z: 1}).Instances()[0].Id() == moduleMap.GetTile(util.Point{X: 1, Y: 1, Z: 1}).Instances()[0].Id() {
		t.Fatal("theme shares instance identity")
	}
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	if p.Modified() {
		t.Fatal("saved project still modified")
	}
	changes, err := p.Changes()
	if err != nil || len(changes) != 0 {
		t.Fatalf("no-op save changed files: %v %d", err, len(changes))
	}
	include, _ := os.ReadFile(d.RootFile)
	if bytes.Count(include, []byte("#include")) != 2 {
		t.Fatalf("incorrect registration includes: %s", include)
	}
	registration, _ := os.ReadFile(p.outputPaths()[1])
	if !strings.Contains(string(registration), `Fresh \[Ship]`) {
		t.Fatalf("unescaped DM name: %s", registration)
	}
	reopened, err := OpenProject(c, d, p.Hull)
	if err != nil {
		t.Fatal(err)
	}
	a, err = reopened.Assemble(reopened.Hull.Themes[0], selection)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Modified() {
		t.Fatal("opening modified sources")
	}
	hullBytes, _ := os.ReadFile(a.Sources[0].File)
	a.Sources[1].Live.GetTile(util.Point{X: 2, Y: 2, Z: 1}).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
	if err = reopened.Save(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(a.Sources[0].File)
	if !bytes.Equal(got, hullBytes) {
		t.Fatal("module edit rewrote hull")
	}
	if err = reopened.Resize(reopened.Hull.Themes[0], 5, 5); err == nil {
		t.Fatal("resize deleted occupied hull")
	}
}

func TestTwoNewShipsShareOneEnvironmentSave(t *testing.T) {
	c, d := authorEnvironment(t)
	a, err := NewProject(c, d, "first", "First", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewProject(c, d, "second", "Second", 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err = SaveProjects([]*Project{a, b}); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(d.RootFile)
	if bytes.Count(content, []byte("#include")) != 4 {
		t.Fatalf("lost environment registration: %s", content)
	}
	if a.Modified() || b.Modified() {
		t.Fatal("saved projects still dirty")
	}
	if err = SaveProjects([]*Project{a, b}); err != nil {
		t.Fatal("second save failed", err)
	}
	again, _ := os.ReadFile(d.RootFile)
	if !bytes.Equal(content, again) {
		t.Fatal("no-op save rewrote includes")
	}
}
