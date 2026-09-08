package wsship

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"sdmm/internal/dmapi/dmenv"
)

// Exercise form validation, editing labels, persistence and the actual undo stack
// in the native rendering fixture, including a freshly parsed handwritten ship.
func exerciseRenaming(t *testing.T, ws *WsShip, render func()) {
	t.Helper()
	ws.setStage(stepBuild)
	ws.selectRoomOption("cargo", "cargo_basic")
	theme, module := ws.currentTheme(), ws.project.Hull.Modules[0]
	for _, target := range []struct {
		task                    buildTask
		id, old, name, typePath string
	}{
		{taskRenameTheme, theme.ID, theme.Name, "Renamed Variant", "/datum/ship_theme/workshop_fixture_" + theme.ID},
		{taskRenameModule, module.ID, module.Name, "Renamed Cargo Bay", "/datum/ship_upgrade_module/workshop_fixture_" + module.ID},
	} {
		ws.beginRename(target.task, target.id, target.old)
		if ws.itemName != target.old || ws.project.RenameNameError(ws.renameScope(), ws.itemName) != nil {
			t.Fatal("rename form did not accept its existing name")
		}
		ws.itemName = "   "
		render()
		ws.applyRename()
		if ws.message == "" || ws.task != target.task {
			t.Fatal("rename form accepted an empty name")
		}
		ws.itemName = target.name
		ws.message = ""
		for i := 0; i < 3; i++ {
			render()
		}
		if dst := os.Getenv("SHIP_RENDER_TEST_OUTPUT"); dst != "" {
			captureFrame(t, filepath.Join(dst, "rename-"+target.id+".png"), 1400, 960)
		}
		ws.applyRename()
		if ws.message != "" || ws.task != taskPaint {
			t.Fatalf("rename did not return to ship editing: %s", ws.message)
		}
		if target.task == taskRenameModule && ws.assembly.Sources[1].Name != target.name {
			t.Fatal("room editing label was not refreshed")
		}
		if !ws.Save() {
			t.Fatal(ws.message)
		}
		fresh, err := dmenv.New(ws.project.Dme.RootFile)
		if err != nil {
			t.Fatal(err)
		}
		if obj := fresh.Objects[target.typePath]; obj == nil || obj.Vars.ValueV("name", "") != strconv.Quote(target.name) {
			t.Fatal("renamed component was not present in the reloaded environment")
		}
		ws.app.CommandStorage().Undo()
		if !ws.Save() {
			t.Fatal(ws.message)
		}
		ws.app.CommandStorage().Redo()
		if !ws.Save() {
			t.Fatal(ws.message)
		}
		// Restore original names for the remaining fixture exercises.
		ws.app.CommandStorage().Undo()
		if !ws.Save() {
			t.Fatal(ws.message)
		}
	}
}
