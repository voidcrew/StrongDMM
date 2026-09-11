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

// addRect keeps the older rectangle call sites readable.
func (p *Project) addRect(themeIndex int, id, name string, lo, hi util.Point) error {
	return p.AddSlot(themeIndex, id, name, lo, FullFootprint(hi.X-lo.X+1, hi.Y-lo.Y+1))
}

// enableFootprints declares the marker variable newer game code ships with.
func enableFootprints(env *dmenv.Dme) {
	v := dmvars.MutableVariables{}
	v.Put(footprintVar, "null")
	env.Objects[SlotMarker] = &dmenv.Object{Path: SlotMarker, Vars: v.ToImmutable()}
}

func pt(x, y int) util.Point { return util.Point{X: x, Y: y, Z: 1} }

func hasPath(tile *dmmap.Tile, path string) bool {
	for _, i := range tile.Instances() {
		if i.Prefab().Path() == path {
			return true
		}
	}
	return false
}

func markerMask(hull *dmmap.Dmm, slot string) (string, bool) {
	_, mask, i := slotMarker(hull, slot)
	return mask, i != nil
}

// The L-shape used below: the top-right and middle-right tiles stay hull.
const lMask = "##../###./####"

func TestFootprintMaskRoundTrip(t *testing.T) {
	f, err := ParseFootprint(lMask, 4, 3)
	if err != nil {
		t.Fatal(err)
	}
	if f.String() != lMask || f.Count() != 9 || f.IsFull() || !f.Connected() {
		t.Fatalf("parsed shape mismatch: %s %d", f.String(), f.Count())
	}
	// Row one of the mask is the top of the module, so y=3 holds "##..".
	if !f.Contains(pt(2, 3)) || f.Contains(pt(3, 3)) || f.Contains(pt(4, 2)) || !f.Contains(pt(4, 1)) {
		t.Fatal("rows are not read from the top down")
	}
	full := FullFootprint(4, 3)
	if !full.IsFull() || full.String() != "####/####/####" {
		t.Fatal("full footprint mask")
	}
	for _, bad := range []string{"", "##/#", "#x", "...", "##/../"} {
		if _, err := parseMask(bad); err == nil {
			t.Fatalf("accepted mask %q", bad)
		}
	}
	if _, err := ParseFootprint(lMask, 3, 3); err == nil {
		t.Fatal("accepted a mask of the wrong size")
	}
	if split, _ := parseMask("#./.#"); split.Connected() {
		t.Fatal("diagonal tiles counted as connected")
	}
	shape, origin, err := FootprintFromTiles([]util.Point{pt(6, 8), pt(5, 8), pt(5, 9)})
	if err != nil || origin != pt(5, 8) || shape.String() != "#./##" {
		t.Fatalf("tiles to footprint: %v %v %s", err, origin, shape.String())
	}
	if _, _, err = FootprintFromTiles(nil); err == nil {
		t.Fatal("accepted an empty selection")
	}
	for id, want := range map[string]string{"cargo_bay": "Cargo bay", "lab": "Lab", "": ""} {
		if got := SlotDisplayName(id); got != want {
			t.Fatalf("%q -> %q", id, got)
		}
	}
}

func shapedProject(t *testing.T) (*Project, *dmmap.Dmm) {
	t.Helper()
	c, env := authorEnvironment(t)
	enableFootprints(env)
	p, err := NewProject(c, env, "shaped", "Shaped", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Deck(p.Hull.Themes[0], pt(2, 2), pt(18, 18)); err != nil {
		t.Fatal(err)
	}
	file, _ := c.HullFile(p.Hull, p.Hull.Themes[0])
	return p, p.Documents[file].Map
}

func TestAddSlotWithShapeLeavesOutsideTilesInHull(t *testing.T) {
	p, hull := shapedProject(t)
	shape, _ := ParseFootprint(lMask, 4, 3)
	origin := pt(5, 5)
	inside, outside := pt(5, 7), pt(8, 7) // top-left is in the room, top-right stays hull
	hull.GetTile(inside).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
	hull.GetTile(outside).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
	before := p.Capture()
	if err := p.AddSlot(0, "cargo_bay", "Cargo Bay", origin, shape); err != nil {
		t.Fatal(err)
	}
	if hasPath(hull.GetTile(inside), "/obj/item/test") || !hasPath(hull.GetTile(outside), "/obj/item/test") {
		t.Fatal("furniture moved regardless of the shape")
	}
	if mask, ok := markerMask(hull, "cargo_bay"); !ok || mask != lMask {
		t.Fatalf("marker footprint %q %v", mask, ok)
	}
	a, err := p.Assemble(p.Hull.Themes[0], map[string]string{"cargo_bay": "cargo_bay_basic"})
	if err != nil {
		t.Fatal(err)
	}
	module := a.Sources[1].Live
	if !hasPath(module.GetTile(pt(1, 3)), "/obj/item/test") || len(module.GetTile(pt(4, 3)).Instances()) != 2 {
		t.Fatal("module content does not follow the shape")
	}
	room := a.Rooms["cargo_bay"]
	if room.Origin != origin || room.Footprint.String() != lMask {
		t.Fatalf("assembly room shape: %+v", room)
	}
	if slot, ok := a.RoomAt(inside); !ok || slot != "cargo_bay" {
		t.Fatal("RoomAt missed an inside tile")
	}
	if _, ok := a.RoomAt(outside); ok {
		t.Fatal("RoomAt claimed a hull tile")
	}
	after := p.Capture()
	p.Restore(before)
	if _, ok := markerMask(hull, "cargo_bay"); ok || !hasPath(hull.GetTile(inside), "/obj/item/test") {
		t.Fatal("undo did not restore the hull")
	}
	p.Restore(after)
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(a.Sources[0].File)
	if !bytes.Contains(saved, []byte(`footprint = "`+lMask+`"`)) {
		t.Fatalf("saved hull lacks the footprint:\n%s", saved)
	}
	reopened, err := OpenProject(p.Catalog, p.Dme, p.Hull)
	if err != nil {
		t.Fatal(err)
	}
	// Without a chosen option the default option still describes the room.
	a, err = reopened.Assemble(reopened.Hull.Themes[0], nil)
	if err != nil || a.Rooms["cargo_bay"].Footprint.String() != lMask {
		t.Fatalf("reopened room shape: %v %+v", err, a.Rooms["cargo_bay"])
	}
	for _, issue := range a.Issues {
		if strings.Contains(issue.Message, "Cargo bay") {
			t.Fatal(issue.Message)
		}
	}
}

func TestRectangleRoomsStayUnchanged(t *testing.T) {
	p, hull := shapedProject(t)
	if err := p.addRect(0, "cargo", "Cargo", pt(4, 4), pt(6, 6)); err != nil {
		t.Fatal(err)
	}
	if mask, ok := markerMask(hull, "cargo"); !ok || mask != "" {
		t.Fatal("rectangle wrote a footprint")
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	file, _ := p.Catalog.HullFile(p.Hull, p.Hull.Themes[0])
	saved, _ := os.ReadFile(file)
	if bytes.Contains(saved, []byte("footprint")) || !bytes.Contains(saved, []byte(`/obj/modular_map_root/ship_upgrade{`+"\n\tkey = \"cargo\"\n\t}")) {
		t.Fatalf("rectangle marker changed:\n%s", saved)
	}
}

func TestShapesNeedGameSupport(t *testing.T) {
	c, env := authorEnvironment(t)
	p, err := NewProject(c, env, "plain", "Plain", 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	if p.SupportsFootprints() {
		t.Fatal("bare environment claims footprint support")
	}
	shape, _ := ParseFootprint(lMask, 4, 3)
	if err = p.AddSlot(0, "bay", "Bay", pt(5, 5), shape); err == nil || !strings.Contains(err.Error(), "update tg-voidcrew") {
		t.Fatalf("shape accepted without support: %v", err)
	}
	if err = p.AddSlot(0, "bay", "Bay", pt(5, 5), FullFootprint(4, 3)); err != nil {
		t.Fatal(err)
	}
	if err = p.ReshapeSlot(0, "bay", pt(5, 5), shape); err == nil || !strings.Contains(err.Error(), "update tg-voidcrew") {
		t.Fatalf("reshape accepted without support: %v", err)
	}
	if err = p.ReshapeSlot(0, "bay", pt(5, 5), FullFootprint(5, 3)); err != nil {
		t.Fatal(err)
	}
}

func TestShapedRoomsInterlockButNeverOverlap(t *testing.T) {
	p, _ := shapedProject(t)
	l, _ := ParseFootprint(lMask, 4, 3)
	if err := p.AddSlot(0, "left", "Left", pt(4, 4), l); err != nil {
		t.Fatal(err)
	}
	// The L leaves 6,6 7,6 and 7,5 as hull; a shape reaching its bottom row fails.
	mirrored, _ := ParseFootprint("##/#./#.", 2, 3)
	if err := p.AddSlot(0, "right", "Right", pt(7, 4), mirrored); err == nil {
		t.Fatal("shape overlapping the first room's bottom row was accepted")
	}
	notch, _ := ParseFootprint("##/.#", 2, 2)
	if err := p.AddSlot(0, "right", "Right", pt(7, 5), notch); err != nil {
		t.Fatalf("interlocking shape rejected: %v", err)
	}
	if err := p.addRect(0, "corner", "Corner", pt(4, 4), pt(4, 4)); err == nil || !strings.Contains(err.Error(), "Left") {
		t.Fatalf("marker tile reuse accepted: %v", err)
	}
	if err := p.addRect(0, "beside", "Beside", pt(9, 4), pt(10, 5)); err != nil {
		t.Fatal(err)
	}
	a, err := p.Assemble(p.Hull.Themes[0], nil)
	if err != nil || len(a.Rooms) != 3 {
		t.Fatalf("rooms: %v %d", err, len(a.Rooms))
	}
	for coord, want := range map[util.Point]string{pt(7, 4): "left", pt(6, 5): "left", pt(7, 6): "right", pt(8, 5): "right", pt(7, 5): "", pt(9, 4): "beside"} {
		if got, _ := a.RoomAt(coord); got != want {
			t.Fatalf("%v is %q, want %q", coord, got, want)
		}
	}
}

func TestRemoveModuleGuardsAndUndo(t *testing.T) {
	p, _ := shapedProject(t)
	if err := p.addRect(0, "cargo", "Cargo", pt(4, 4), pt(6, 6)); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveModule("cargo_basic"); err == nil || !strings.Contains(err.Error(), "only option") {
		t.Fatalf("only option removed: %v", err)
	}
	if err := p.AddModule(0, p.Hull.Modules[0], "cargo_empty", "Empty Cargo", true); err != nil {
		t.Fatal(err)
	}
	if err := p.SetPartCosts("module/cargo_empty", PartCosts{"trade": 2}); err != nil {
		t.Fatal(err)
	}
	crewTestTypes(p)
	if err := p.SetCrewJobs("module/cargo_empty", []CrewJob{{Name: "Loader", Outfit: "/datum/outfit/job/assistant", Category: "Cargo", Slots: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveModule("cargo_basic"); err == nil || !strings.Contains(err.Error(), "another default") {
		t.Fatalf("default removed: %v", err)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	file, _ := p.moduleFile(p.Hull.Modules[1], "standard")
	before := p.Capture()
	if err := p.RemoveModule("cargo_empty"); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Crew.Rosters["module/cargo_empty"]; ok || len(p.Hull.Modules) != 1 || p.partCosts["module/cargo_empty"] != nil || !p.Modified() {
		t.Fatal("option not unregistered")
	}
	after := p.Capture()
	p.Restore(before)
	if _, ok := p.Crew.Rosters["module/cargo_empty"]; !ok || len(p.Hull.Modules) != 2 || !p.Documents[file].Active || p.Modified() {
		t.Fatal("undo did not bring the option back")
	}
	p.Restore(after)
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("removed option map remains")
	}
	registration, _ := os.ReadFile(p.outputPaths()[2])
	if bytes.Contains(registration, []byte("cargo_empty")) {
		t.Fatal("removed option still registered")
	}
	p.Restore(before)
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("undo after save did not restore the option map")
	}
	if err := p.SetDefaultModule("cargo", "cargo_empty"); err != nil {
		t.Fatal(err)
	}
	if p.Hull.Modules[0].Default || !p.Hull.Modules[1].Default {
		t.Fatal("default flag not moved")
	}
	if err := p.RemoveModule("cargo_basic"); err != nil {
		t.Fatal(err)
	}
	if err := p.SetDefaultModule("cargo", "missing"); err == nil {
		t.Fatal("unknown option became default")
	}
}

func TestRemoveSlotRestoresHullContent(t *testing.T) {
	p, hull := shapedProject(t)
	shape, _ := ParseFootprint(lMask, 4, 3)
	origin, inside := pt(5, 5), pt(5, 7)
	hull.GetTile(inside).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
	if err := p.AddSlot(0, "cargo", "Cargo", origin, shape); err != nil {
		t.Fatal(err)
	}
	if err := p.AddModule(0, p.Hull.Modules[0], "cargo_empty", "Empty", true); err != nil {
		t.Fatal(err)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, m := range p.Hull.Modules {
		f, _ := p.moduleFile(m, "standard")
		files = append(files, f)
	}
	before := p.Capture()
	if err := p.RemoveSlot(0, "cargo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := markerMask(hull, "cargo"); ok || !hasPath(hull.GetTile(inside), "/obj/item/test") {
		t.Fatal("hull content or marker not restored")
	}
	if len(p.Hull.Slots) != 0 || len(p.Hull.Themes[0].Slots) != 0 || len(p.Hull.Modules) != 0 {
		t.Fatalf("slot still registered: %+v", p.Hull)
	}
	a, err := p.Assemble(p.Hull.Themes[0], nil)
	if err != nil || len(a.Markers) != 0 || len(a.Rooms) != 0 {
		t.Fatalf("assembly still has the room: %v", err)
	}
	after := p.Capture()
	p.Restore(before)
	if _, ok := markerMask(hull, "cargo"); !ok || hasPath(hull.GetTile(inside), "/obj/item/test") || len(p.Hull.Modules) != 2 {
		t.Fatal("undo did not restore the room")
	}
	p.Restore(after)
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("option map remains: %s", f)
		}
	}
	registration, _ := os.ReadFile(p.outputPaths()[1])
	if !bytes.Contains(registration, []byte("upgrade_slot_ids = list()")) {
		t.Fatalf("slot list not emptied:\n%s", registration)
	}
	if err = p.RemoveSlot(0, "cargo"); err == nil {
		t.Fatal("removed a missing room")
	}
	p.Restore(before)
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("undo after save lost %s", f)
		}
	}
}

// Registers the fixture's saved room option like the environment would.
func registerFixtureModules(p *Project, source string) {
	for _, m := range p.Hull.Modules {
		vars := dmvars.MutableVariables{}
		vars.Put("id", dmQuote(m.ID))
		vars.Put("for_ship", p.Hull.Type)
		path := "/datum/ship_upgrade_module/loaded_rooms_" + m.ID
		p.Dme.Objects[path] = &dmenv.Object{Path: path, Vars: vars.ToImmutable(), Location: sdmmparser.Location{File: source}}
	}
}

func TestRemoveSlotFromHandwrittenShip(t *testing.T) {
	p, sourceFile := loadedRoomProject(t, true)
	registerFixtureModules(p, sourceFile)
	hullSource, _ := p.roomTypeFile(p.Hull.Type)
	originalSource, _ := os.ReadFile(sourceFile)
	originalHull, _ := os.ReadFile(hullSource)
	var files []string
	for _, theme := range p.Hull.Themes {
		f, _ := p.moduleFile(p.Hull.Modules[0], theme.ID)
		files = append(files, f)
	}
	before := p.Capture()
	if err := p.RemoveSlot(0, "original"); err != nil {
		t.Fatal(err)
	}
	if len(p.Hull.Modules) != 0 || Contains(p.Hull.Slots, "original") || Contains(p.Hull.Themes[1].Slots, "original") {
		t.Fatal("slot remains registered")
	}
	for _, theme := range p.Hull.Themes {
		file, _ := p.Catalog.HullFile(p.Hull, theme)
		d, _ := p.document(file)
		if _, ok := markerMask(d.Map, "original"); ok {
			t.Fatalf("%s hull keeps the marker", theme.Name)
		}
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(sourceFile)
	text := string(saved)
	if strings.Contains(text, "/datum/ship_upgrade_module/loaded_rooms_original_basic") || !strings.Contains(text, "custom_proc()") || strings.Count(text, "upgrade_slot_ids = list()") != 2 {
		t.Fatalf("handwritten source not rewritten:\n%s", text)
	}
	savedHull, _ := os.ReadFile(hullSource)
	if !strings.Contains(string(savedHull), "\tupgrade_slot_ids = list()\n") || !strings.Contains(string(savedHull), "has_upgrade_slots = TRUE") {
		t.Fatalf("hull definition not rewritten:\n%s", savedHull)
	}
	for _, f := range files {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("option map remains: %s", f)
		}
	}
	if p.Modified() {
		t.Fatal("saved removal remains dirty")
	}
	p.Restore(before)
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(sourceFile)
	restoredHull, _ := os.ReadFile(hullSource)
	if !bytes.Equal(restored, originalSource) || !bytes.Equal(restoredHull, originalHull) {
		t.Fatalf("undo after save did not restore the definitions:\n%s", restored)
	}
	for _, f := range files {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("undo after save lost %s", f)
		}
	}
}

func TestHandwrittenDefaultAndOptionRemoval(t *testing.T) {
	p, sourceFile := loadedRoomProject(t, true)
	registerFixtureModules(p, sourceFile)
	if err := p.AddModule(0, p.Hull.Modules[0], "spare", "Spare", true); err != nil {
		t.Fatal(err)
	}
	if err := p.SetDefaultModule("original", "spare"); err != nil {
		t.Fatal(err)
	}
	if err := p.RemoveModule("original_basic"); err != nil {
		t.Fatal(err)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(sourceFile)
	if bytes.Contains(saved, []byte("loaded_rooms_original_basic")) || !bytes.Contains(saved, []byte("custom_proc()")) {
		t.Fatalf("handwritten option not removed:\n%s", saved)
	}
	code, _ := os.ReadFile(p.rooms.code)
	if !bytes.Contains(code, []byte("workshop_loaded_rooms_spare")) || !bytes.Contains(code, []byte("is_default = TRUE")) {
		t.Fatalf("new default not registered:\n%s", code)
	}
}

func TestHandwrittenDefaultFlagRewrite(t *testing.T) {
	p, sourceFile := loadedRoomProject(t, true)
	registerFixtureModules(p, sourceFile)
	if err := p.AddModule(0, p.Hull.Modules[0], "spare", "Spare", true); err != nil {
		t.Fatal(err)
	}
	if err := p.SetDefaultModule("original", "spare"); err != nil {
		t.Fatal(err)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(sourceFile)
	block := string(saved[bytes.Index(saved, []byte("/datum/ship_upgrade_module/loaded_rooms_original_basic")):])
	if next := strings.Index(block, "\n/"); next > 0 {
		block = block[:next]
	}
	if !strings.Contains(block, "is_default = FALSE") || strings.Contains(block, "is_default = TRUE") {
		t.Fatalf("old default keeps its flag:\n%s", block)
	}
}

func TestReshapeSlotGrowsShrinksAndShifts(t *testing.T) {
	p, hull := shapedProject(t)
	if err := p.addRect(0, "cargo", "Cargo", pt(5, 5), pt(7, 7)); err != nil {
		t.Fatal(err)
	}
	if err := p.AddModule(0, p.Hull.Modules[0], "cargo_empty", "Empty Cargo", true); err != nil {
		t.Fatal(err)
	}
	file, _ := p.moduleFile(p.Hull.Modules[0], "standard")
	module := p.Documents[file].Map
	module.GetTile(pt(3, 3)).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test")) // hull 7,7
	hull.GetTile(pt(4, 4)).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))   // joins the room
	before := p.Capture()
	if err := p.ReshapeSlot(0, "cargo", pt(4, 4), FullFootprint(4, 4)); err != nil {
		t.Fatal(err)
	}
	if module.MaxX != 4 || module.MaxY != 4 || !hasPath(module.GetTile(pt(4, 4)), "/obj/item/test") || !hasPath(module.GetTile(pt(1, 1)), Connector) {
		t.Fatal("grown option lost its content or connector")
	}
	if !hasPath(module.GetTile(pt(1, 1)), "/obj/item/test") || hasPath(hull.GetTile(pt(4, 4)), "/obj/item/test") {
		t.Fatal("added tile kept its furniture in the hull")
	}
	empty, _ := p.moduleFile(p.Hull.Modules[1], "standard")
	if m := p.Documents[empty].Map; m.MaxX != 4 || !hasPath(m.GetTile(pt(1, 1)), Connector) {
		t.Fatal("other option not re-dimensioned")
	}
	coord, mask, marker := slotMarker(hull, "cargo")
	if marker == nil || coord != pt(4, 4) || mask != "" {
		t.Fatalf("marker after growth: %v %q", coord, mask)
	}
	a, err := p.Assemble(p.Hull.Themes[0], map[string]string{"cargo": "cargo_basic"})
	if err != nil {
		t.Fatal(err)
	}
	if room := a.Rooms["cargo"]; room.Origin != pt(4, 4) || room.Footprint.W != 4 {
		t.Fatalf("assembled room: %+v", room)
	}
	// Shrinking over the item is refused by name; shrinking around it works.
	if err = p.ReshapeSlot(0, "cargo", pt(4, 4), FullFootprint(3, 3)); err == nil || !strings.Contains(err.Error(), "Cargo still has items") {
		t.Fatalf("shrink over content: %v", err)
	}
	notch, _ := ParseFootprint(".###/####/####/####", 4, 4)
	if err = p.ReshapeSlot(0, "cargo", pt(4, 4), notch); err != nil {
		t.Fatal(err)
	}
	if _, mask, _ = slotMarker(hull, "cargo"); mask != notch.String() {
		t.Fatalf("marker mask %q", mask)
	}
	if err = p.ReshapeSlot(0, "cargo", pt(18, 18), FullFootprint(4, 4)); err == nil {
		t.Fatal("box outside the hull accepted")
	}
	p.Restore(before)
	if module.MaxX != 3 || !hasPath(module.GetTile(pt(3, 3)), "/obj/item/test") || !hasPath(hull.GetTile(pt(4, 4)), "/obj/item/test") {
		t.Fatal("undo did not restore the option box")
	}
	if coord, _, _ = slotMarker(hull, "cargo"); coord != pt(5, 5) {
		t.Fatal("undo did not move the marker back")
	}
	if err = p.Save(); err != nil || p.Modified() {
		t.Fatalf("save after undo: %v", err)
	}
}

func TestReshapeCannotEnterAnotherRoom(t *testing.T) {
	p, _ := shapedProject(t)
	if err := p.addRect(0, "left", "Left", pt(4, 4), pt(5, 5)); err != nil {
		t.Fatal(err)
	}
	if err := p.addRect(0, "right", "Right", pt(6, 4), pt(7, 5)); err != nil {
		t.Fatal(err)
	}
	if err := p.ReshapeSlot(0, "left", pt(4, 4), FullFootprint(3, 2)); err == nil || !strings.Contains(err.Error(), "Right") {
		t.Fatalf("overlap accepted: %v", err)
	}
	if err := p.ReshapeSlot(0, "left", pt(3, 4), FullFootprint(3, 2)); err != nil {
		t.Fatal(err)
	}
	if err := p.ReshapeSlot(0, "missing", pt(3, 4), FullFootprint(3, 2)); err == nil {
		t.Fatal("reshaped a missing room")
	}
}
