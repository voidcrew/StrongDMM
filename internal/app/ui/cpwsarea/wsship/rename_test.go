package wsship

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
		ws.itemDescription = "A medical \"bay\" with supplies.\nReady for longer trips."
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
		if got := fresh.Objects[target.typePath].Vars.ValueV("desc", ""); got != strconv.Quote(ws.itemDescription) {
			t.Fatalf("description did not survive Save and environment reload: %s", got)
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
	// Save also has to include text still in the form, without an Apply click.
	theme = ws.currentTheme()
	ws.beginRename(taskRenameTheme, theme.ID, theme.Name)
	ws.itemName = "Variant saved directly from form"
	ws.itemDescription = "Description saved directly from form"
	if !ws.IsModified() || !strings.HasPrefix(ws.Name(), "* ") {
		t.Fatal("variant name draft has no unsaved indicator")
	}
	if !ws.Save() {
		t.Fatal(ws.message)
	}
	fresh, err := dmenv.New(ws.project.Dme.RootFile)
	if err != nil {
		t.Fatal(err)
	}
	path := "/datum/ship_theme/workshop_fixture_" + theme.ID
	if obj := fresh.Objects[path]; obj == nil || obj.Vars.ValueV("name", "") != strconv.Quote("Variant saved directly from form") {
		t.Fatal("Save discarded the variant name still being edited in its form")
	}
	if fresh.Objects[path].Vars.ValueV("desc", "") != strconv.Quote(ws.itemDescription) {
		t.Fatal("Save discarded the description draft")
	}
	ws.app.CommandStorage().Undo()
	if !ws.Save() {
		t.Fatal(ws.message)
	}
	ws.beginRename(taskRenameTheme, theme.ID, theme.Name)
	ws.itemName = ""
	if ws.Save() || ws.task != taskRenameTheme {
		t.Fatal("Save accepted or discarded an invalid variant name")
	}
	ws.itemName = "Cancelled variant name"
	ws.itemDescription = "Cancelled description"
	ws.cancelRename()
	if ws.IsModified() || ws.currentTheme().Name != theme.Name || ws.project.Description("theme/"+theme.ID) == "Cancelled description" {
		t.Fatal("Cancel applied the variant name draft")
	}
	ws.beginRename(taskRenameTheme, theme.ID, theme.Name)
	ws.itemDescription = "Only the description changes"
	if !ws.IsModified() || !ws.Save() || ws.currentTheme().Name != theme.Name {
		t.Fatalf("description-only draft did not save: %s", ws.message)
	}
	ws.beginRename(taskRenameTheme, theme.ID, theme.Name)
	ws.itemDescription = ""
	if !ws.Save() {
		t.Fatal(ws.message)
	}
	fresh, err = dmenv.New(ws.project.Dme.RootFile)
	if err != nil || fresh.Objects[path].Vars.ValueV("desc", "missing") != `""` {
		t.Fatalf("clearing the description did not persist: %v", err)
	}
	ws.app.CommandStorage().Undo()
	ws.app.CommandStorage().Undo()
	if !ws.Save() {
		t.Fatal(ws.message)
	}
}
