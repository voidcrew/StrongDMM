package ship

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
	"sdmm/third_party/sdmmparser"
)

func loadedRoomProject(t *testing.T, themed bool) (*Project, string) {
	t.Helper()
	c, env := authorEnvironment(t)
	p, err := NewProject(c, env, "loaded_rooms", "Loaded Rooms", 24, 24)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Deck(p.Hull.Themes[0], util.Point{X: 3, Y: 3, Z: 1}, util.Point{X: 18, Y: 18, Z: 1}); err != nil {
		t.Fatal(err)
	}
	if err = p.AddSlot(0, "original", "Original Room", util.Point{X: 3, Y: 3, Z: 1}, util.Point{X: 5, Y: 5, Z: 1}); err != nil {
		t.Fatal(err)
	}
	if err = p.AddTheme(0, "other", "Other Variant"); err != nil {
		t.Fatal(err)
	}
	paths := p.outputPaths()
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(paths[0]); err != nil {
		t.Fatal(err)
	}
	// These now represent handwritten definitions, with jobs and prices that
	// must survive editing. Their source locations come from the environment.
	source, _ := os.ReadFile(paths[2])
	source = append(source, []byte("\n// Keep the ship's custom jobs and prices.\n/datum/ship_theme/loaded_rooms_other/custom_proc()\n\treturn 17\n")...)
	if err = os.WriteFile(paths[2], source, 0600); err != nil {
		t.Fatal(err)
	}
	h := cloneHull(p.Hull)
	if !themed {
		h.Themes = nil
	}
	for _, theme := range h.Themes {
		vars := dmvars.MutableVariables{}
		vars.Put("id", dmQuote(theme.ID))
		vars.Put("for_ship", h.Type)
		path := "/datum/ship_theme/loaded_rooms_" + theme.ID
		env.Objects[path] = &dmenv.Object{Path: path, Vars: vars.ToImmutable(), Location: sdmmparser.Location{File: paths[2]}}
	}
	env.Objects[h.Type] = &dmenv.Object{Path: h.Type, Vars: (&dmvars.MutableVariables{}).ToImmutable(), Location: sdmmparser.Location{File: paths[1]}}
	p, err = OpenProject(c, env, h)
	if err != nil {
		t.Fatal(err)
	}
	if p.Settings != nil || p.Modified() {
		t.Fatal("loaded ship was converted or marked dirty")
	}
	return p, paths[2]
}

func TestLoadedShipRoomCreationAndSaveUndo(t *testing.T) {
	p, sourceFile := loadedRoomProject(t, true)
	beforeSource, _ := os.ReadFile(sourceFile)
	hullFile, _ := p.Catalog.HullFile(p.Hull, p.Hull.Themes[0])
	beforeHull, _ := os.ReadFile(hullFile)
	otherFile, _ := p.Catalog.HullFile(p.Hull, p.Hull.Themes[1])
	otherBytes, _ := os.ReadFile(otherFile)
	lo, hi := util.Point{X: 10, Y: 7, Z: 1}, util.Point{X: 12, Y: 9, Z: 1}
	doc, _ := p.document(hullFile)
	doc.Map.GetTile(lo).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
	doc.Map.GetTile(lo).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/machinery/power/apc"))
	before := p.Capture()
	if err := p.AddSlot(0, "new_room", "New Room", lo, hi); err != nil {
		t.Fatal(err)
	}
	if Contains(p.Hull.Slots, "new_room") || Contains(p.Hull.SlotsFor(p.Hull.Themes[1]), "new_room") {
		t.Fatal("new room leaked to other variants")
	}
	if !p.Modified() {
		t.Fatal("new room is not marked dirty")
	}
	base := p.Hull.Modules[len(p.Hull.Modules)-1]
	if err := p.AddModule(0, base, "medical_option", "Medical Option", true); err != nil {
		t.Fatal(err)
	}
	a, err := p.Assemble(p.Hull.Themes[0], map[string]string{"new_room": base.ID})
	if err != nil || len(a.Sources) != 2 {
		t.Fatalf("could not assemble new room: %v", err)
	}
	item, apc := false, false
	for _, atom := range a.Cells[lo] {
		if atom.Prefab.Path() == "/obj/item/test" {
			item = atom.Source == 1
		}
		if atom.Prefab.Path() == "/obj/machinery/power/apc" {
			apc = atom.Source == 0
		}
	}
	if !item || !apc {
		t.Fatal("extraction changed permanent hull equipment")
	}
	after := p.Capture()
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	savedSource, _ := os.ReadFile(sourceFile)
	if bytes.Equal(savedSource, beforeSource) || !bytes.Contains(savedSource, []byte("custom_proc()")) {
		t.Fatal("slot list was not saved or custom code was lost")
	}
	if p.Modified() {
		t.Fatal("saved loaded ship remains dirty")
	}
	currentOther, _ := os.ReadFile(otherFile)
	if !bytes.Equal(otherBytes, currentOther) {
		t.Fatal("other variant map changed")
	}
	code, _ := os.ReadFile(p.rooms.code)
	if bytes.Count(code, []byte("/datum/ship_upgrade_module/")) != 2 {
		t.Fatal("new options missing or original options regenerated")
	}
	p.Restore(before)
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	restoredSource, _ := os.ReadFile(sourceFile)
	if !bytes.Equal(restoredSource, beforeSource) {
		t.Fatal("undo after saving did not restore handwritten definitions")
	}
	p.Restore(after)
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenProject(p.Catalog, p.Dme, p.Hull)
	if err != nil || reopened.Settings != nil || reopened.Modified() {
		t.Fatalf("failed to reopen loaded ship: %v", err)
	}
	a, err = reopened.Assemble(reopened.Hull.Themes[0], map[string]string{"new_room": "medical_option"})
	if err != nil || len(a.Sources) != 2 {
		t.Fatalf("saved option does not assemble: %v", err)
	}
	finalHull, _ := os.ReadFile(hullFile)
	if bytes.Equal(beforeHull, finalHull) {
		t.Fatal("hull marker was not saved")
	}
}

func TestLoadedShipRoomNamesAndExternalEdits(t *testing.T) {
	p, sourceFile := loadedRoomProject(t, true)
	lo, hi := util.Point{X: 10, Y: 7, Z: 1}, util.Point{X: 12, Y: 9, Z: 1}
	if err := p.AddSlot(0, "added", "Added", lo, hi); err != nil {
		t.Fatal(err)
	}
	if err := p.AddSlot(1, "added", "Another Name", lo, hi); err == nil {
		t.Fatal("duplicate theme-local slot ID accepted")
	}
	if err := p.AddModule(0, p.Hull.Modules[0], "duplicate_name", "  ADDED  ", true); err == nil {
		t.Fatal("duplicate room name accepted")
	}
	before, _ := os.ReadFile(sourceFile)
	changed := append(before, []byte("\n// External edit\n")...)
	if err := os.WriteFile(sourceFile, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.Save(); err == nil || !strings.Contains(err.Error(), "changed outside") {
		t.Fatalf("external edit was not protected: %v", err)
	}
	if _, err := os.Stat(p.rooms.code); !os.IsNotExist(err) {
		t.Fatal("failed save wrote new module definitions")
	}
}

func TestLoadedShipWithoutThemesCanAddRooms(t *testing.T) {
	p, _ := loadedRoomProject(t, false)
	lo, hi := util.Point{X: 10, Y: 7, Z: 1}, util.Point{X: 12, Y: 9, Z: 1}
	if err := p.AddSlot(0, "unthemed_room", "Unthemed Room", lo, hi); err != nil {
		t.Fatal(err)
	}
	a, err := p.Assemble(Theme{}, map[string]string{"unthemed_room": "unthemed_room_basic"})
	if err != nil || len(a.Sources) != 2 {
		t.Fatalf("unthemed room unavailable before save: %v", err)
	}
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	code, _ := os.ReadFile(p.rooms.code)
	if !bytes.Contains(code, []byte("for_theme = null")) {
		t.Fatal("unthemed option has a theme restriction")
	}
}
