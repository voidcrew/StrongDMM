package wsship

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/ship"
	"sdmm/internal/util"
)

// Reopen the isolated fixture as a handwritten ship, then use the same native
// selection/action/save path used for existing fleet ships. Production is read-only.
func exerciseLoadedShipRooms(t *testing.T, ws *WsShip, render func()) {
	t.Helper()
	root := ws.catalog.Root
	metadata := filepath.Join(root, "voidcrew/mapping/ship_projects/workshop_fixture.ship.json")
	if err := os.Remove(metadata); err != nil {
		t.Fatal(err)
	}
	registration := filepath.Join(root, "voidcrew/mapping/shuttles/workshop_fixture.dm")
	beforeRegistration, err := os.ReadFile(registration)
	if err != nil {
		t.Fatal(err)
	}
	custom := bytes.Replace(beforeRegistration, []byte(`catalog_desc = ""`), []byte(`catalog_desc = "Handwritten ship with custom jobs and costs"`), 1)
	custom = bytes.Replace(custom, []byte("player_hidden = TRUE"), []byte("player_hidden = FALSE"), 1)
	if err = os.WriteFile(registration, custom, 0600); err != nil {
		t.Fatal(err)
	}
	environment, err := dmenv.New(ws.project.Dme.RootFile)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ship.OpenProject(ws.catalog, environment, ws.project.Hull)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Settings != nil {
		t.Fatal("fixture did not reopen as a handwritten ship")
	}
	ws.app.(*previewApp).dme = environment
	ws.projects[loaded.Hull.Type] = loaded
	ws.defaults()
	ws.rebuild()
	ws.setStage(stepBuild)
	ws.OnFocusChange(true)
	lo, hi := util.Point{X: 15, Y: 6, Z: 1}, util.Point{X: 17, Y: 8, Z: 1}
	// Native fixture setup: mapped floors/areas remain in the hull during extraction.
	hull := ws.assembly.Sources[0].Live
	ws.change("Map another room", func() error {
		for y := lo.Y; y <= hi.Y; y++ {
			for x := lo.X; x <= hi.X; x++ {
				tile := hull.GetTile(util.Point{X: x, Y: y, Z: 1})
				tile.InstancesRemoveByPath("/turf")
				tile.InstancesAdd(dmmap.PrefabStorage.Initial("/turf/open/floor/plating"))
				tile.InstancesRemoveByPath("/area")
				tile.InstancesAdd(dmmap.PrefabStorage.Initial("/area/shuttle/voidcrew/workshop_fixture"))
			}
		}
		hull.GetTile(lo).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/structure/table"))
		return nil
	})
	if !tools.SetGrabSelection(lo, hi) {
		t.Fatal("could not select room on loaded ship")
	}
	ws.beginTask(taskRoom)
	ws.itemName = "Loaded Bay"
	for i := 0; i < 3; i++ {
		render()
	}
	if dst := os.Getenv("SHIP_RENDER_TEST_OUTPUT"); dst != "" {
		captureFrame(t, filepath.Join(dst, "loaded-ship-new-room.png"), 1400, 960)
	}
	ws.applyRegion()
	if ws.message != "" || ws.source == 0 {
		t.Fatalf("loaded ship could not create and edit room: %s", ws.message)
	}
	if ws.assembly.Sources[ws.source].Name != "Loaded Bay" {
		t.Fatal("new room is not the editing target")
	}
	ws.app.CommandStorage().Undo()
	if ship.Contains(loaded.Hull.SlotsFor(loaded.Hull.Themes[0]), "loaded_bay") {
		t.Fatal("undo kept loaded ship room")
	}
	ws.app.CommandStorage().Redo()
	if err = loaded.Save(); err != nil {
		t.Fatal(err)
	}
	afterRegistration, _ := os.ReadFile(registration)
	if !bytes.Equal(custom, afterRegistration) {
		t.Fatal("room creation rewrote the ship's custom definition")
	}
	// Discover through a freshly parsed environment, without authoring metadata.
	config, _ := strconv.Unquote(environment.Objects[ship.SlotMarker].Vars.ValueV("config_file", ""))
	configPath, err := ship.Inside(root, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(configPath, []byte("directory = "+strconv.Quote(ws.catalog.ModuleDir)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fresh, err := dmenv.New(environment.RootFile)
	if err != nil {
		t.Fatal("saved loaded-ship room definitions did not parse:", err)
	}
	// Inspect datums directly as well: discovery cannot use the removed .ship.json.
	foundRoom, foundTheme := false, false
	for path, obj := range fresh.Objects {
		if strings.HasPrefix(path, "/datum/ship_upgrade_module/") && obj.Vars.ValueV("id", "") == `"loaded_bay_basic"` {
			foundRoom = obj.Vars.ValueV("for_ship", "") == loaded.Hull.Type
		}
		if path == "/datum/ship_theme/workshop_fixture_standard" {
			foundTheme = strings.Contains(obj.Vars.ValueV("upgrade_slot_ids", ""), `"loaded_bay"`)
		}
	}
	if !foundRoom || !foundTheme {
		t.Fatal("fresh environment lost the new room or its theme registration")
	}
	if ship.Contains(loaded.Hull.SlotsFor(loaded.Hull.Themes[1]), "loaded_bay") {
		t.Fatal("room leaked into the other variant")
	}
	discovered, err := ship.Discover(fresh)
	if err != nil {
		t.Fatal(err)
	}
	var savedHull ship.Hull
	for _, hull := range discovered.Hulls {
		if hull.Type == loaded.Hull.Type {
			savedHull = hull
		}
	}
	if savedHull.Type == "" {
		t.Fatal("loaded ship disappeared after rediscovery")
	}
	reopened, err := ship.OpenProject(discovered, fresh, savedHull)
	if err != nil {
		t.Fatal(err)
	}
	a, err := reopened.Assemble(reopened.Hull.Themes[0], map[string]string{"loaded_bay": "loaded_bay_basic"})
	if err != nil || len(a.Sources) != 2 {
		t.Fatalf("new room did not reopen: %v", err)
	}
}
