package wsship

import (
	"bytes"
	"os"
	"path/filepath"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/ship"
	"sdmm/internal/util"
	"testing"
)

// Called inside the native GL test so the same pane, snapshots and command
// storage used by the application execute the whole authoring workflow.
func exerciseAuthoring(t *testing.T, ws *WsShip, dme *dmenv.Dme, render func()) {
	t.Helper()
	root := os.Getenv("SHIP_AUTHOR_FIXTURE_OUTPUT")
	if root == "" {
		root = t.TempDir()
	}
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	draftEnv := *dme
	draftEnv.RootDir = root
	draftEnv.RootFile = filepath.Join(root, "workshop.dme")
	if err := os.WriteFile(draftEnv.RootFile, []byte("#include \""+filepath.ToSlash(dme.RootFile)+"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	catalog := &ship.Catalog{Root: root, ModuleDir: "_maps/voidcrew/ship_modules/"}
	project, err := ship.NewProject(catalog, &draftEnv, "workshop_fixture", "Workshop Fixture", 24, 24)
	if err != nil {
		t.Fatal(err)
	}
	ws.projects[project.Hull.Type] = project
	ws.catalog.Hulls = append(ws.catalog.Hulls, project.Hull)
	ws.hull = len(ws.catalog.Hulls) - 1
	ws.theme = 0
	ws.defaults()
	ws.rebuild()
	if ws.message != "" {
		t.Fatal(ws.message)
	}
	lo, hi := util.Point{X: 4, Y: 5, Z: 1}, util.Point{X: 12, Y: 13, Z: 1}
	ws.change("Lay permanent deck", func() error { return project.Deck(ws.currentTheme(), lo, hi) })
	ws.change("Extract cargo", func() error { return project.AddSlot(0, "cargo", "Cargo", lo, hi) })
	if ws.message != "" {
		t.Fatal(ws.message)
	}
	ws.defaults()
	ws.rebuild()
	ws.source = 1
	ws.rebuild()
	ws.OnFocusChange(true)
	native := ws.pane.Dmm()
	hull := ws.assembly.Sources[0].Live
	beforeHull := ship.RawData(hull).EncodeTGM()
	coord := util.Point{X: 2, Y: 3, Z: 1}
	prefab := dmmap.PrefabStorage.Initial("/obj/structure/table")
	ws.pane.Editor().TileReplace(coord, dmmdata.Prefabs{prefab, dmmap.PrefabStorage.Initial("/turf/template_noop"), dmmap.PrefabStorage.Initial("/area/template_noop")})
	ws.pane.Editor().CommitContextNow("Paint table")
	if !bytes.Equal(beforeHull, ship.RawData(hull).EncodeTGM()) {
		t.Fatal("module edit changed hull")
	}
	global := util.Point{X: lo.X + coord.X - 1, Y: lo.Y + coord.Y - 1, Z: 1}
	found := false
	for _, atom := range ws.assembly.Cells[global] {
		if atom.Prefab.Path() == prefab.Path() {
			found = true
			if atom.Local != coord || atom.Source != 1 {
				t.Fatal("incorrect owner or local coordinate")
			}
		}
	}
	if !found {
		t.Fatal("source edit did not appear in live assembly")
	}
	// Switching away before undo must focus the owner and update the scene.
	ws.source = 0
	ws.rebuild()
	ws.app.CommandStorage().Undo()
	if ws.pane.Dmm() != native {
		t.Fatal("undo did not activate its source")
	}
	for _, atom := range ws.assembly.Cells[global] {
		if atom.Prefab.Path() == prefab.Path() {
			t.Fatal("undo kept painted object")
		}
	}
	ws.app.CommandStorage().Redo()
	ws.change("Clone theme", func() error { return project.AddTheme(0, "pirate", "Pirate") })
	if ws.message != "" {
		t.Fatal(ws.message)
	}
	ws.theme = 1
	ws.defaults()
	ws.rebuild()
	if ws.assembly.Sources[1].Live == native {
		t.Fatal("theme shares source")
	}
	ws.app.CommandStorage().Undo() // theme creation
	ws.app.CommandStorage().Undo() // the earlier source edit still has valid history
	ws.app.CommandStorage().Redo()
	ws.app.CommandStorage().Redo()
	ws.theme = 0
	ws.defaults()
	ws.rebuild()
	ws.source = 1
	ws.rebuild()
	// Save just the isolated fixture, never production maps.
	ws.flush()
	if err := project.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := ship.OpenProject(catalog, &draftEnv, project.Hull)
	if err != nil {
		t.Fatal(err)
	}
	assembled, err := reopened.Assemble(reopened.Hull.Themes[0], map[string]string{"cargo": "cargo_basic"})
	if err != nil {
		t.Fatal(err)
	}
	if len(assembled.Sources) != 2 {
		t.Fatal("reopen lost module")
	}
	ws.pane.FitView()
	render()
	if dst := os.Getenv("SHIP_RENDER_TEST_OUTPUT"); dst != "" {
		captureFrame(t, filepath.Join(dst, "new-ship-workshop.png"), 1400, 960)
	}
}
