package wsplanet

import (
	"os"
	"reflect"
	"testing"

	"sdmm/internal/planet"
)

func testPlanetDraftNavigation(t *testing.T, w *Workspace, capture func(string)) {
	t.Helper()
	original := w.project
	before := planet.Clone(original.State)
	defer func() {
		original.State = before
		w.bind(original)
		w.creating, w.cancelling = false, false
	}()
	w.startCreation()
	w.newName = "Temporary planet test"
	w.blank = true
	w.createPlanet()
	draft := w.project
	if !draft.UnsavedNew() {
		t.Fatal("new planet was not created", w.message)
	}
	path := draft.State.Definition.Path
	draft.State.Seeds.Height++
	w.record()
	changed := planet.Clone(draft.State)
	w.biomeName, w.nameDirty = "Unfinished biome name", true
	w.switchPlanet(original.State.Definition.Path)
	if w.project != original || !w.IsModified() {
		t.Fatal("could not leave an unsaved new planet")
	}
	original.State.Seeds.Moisture++
	w.record()
	w.switchPlanet(path)
	if !reflect.DeepEqual(w.project.State, changed) || !w.nameDirty || w.biomeName != "Unfinished biome name" {
		t.Fatal("switching lost the new planet draft")
	}
	w.nameDirty = false
	w.biomeName = w.project.State.Biomes[w.selected].Name
	w.app.CommandStorage().Undo()
	if draft.State.Seeds.Height != before.Seeds.Height || original.State.Seeds.Moisture != before.Seeds.Moisture+1 {
		t.Fatal("undo affected the wrong planet")
	}
	w.app.CommandStorage().Redo()
	if !reflect.DeepEqual(draft.State, changed) {
		t.Fatal("new planet undo history was lost on switch")
	}
	if len(w.planetChoices()) != len(w.catalog.Planets)+1 {
		t.Fatal("unsaved planet is missing from Open planet choices")
	}
	capture("new-planet-draft")
	w.cancelling = true
	capture("cancel-new-planet")
	w.discardNewPlanet()
	if w.project != original || w.drafts[path] != nil || original.State.Seeds.Moisture != before.Seeds.Moisture+1 {
		t.Fatal("cancellation discarded another planet or kept the new one")
	}
	if _, err := os.Stat(draft.GeneratedPath()); !os.IsNotExist(err) {
		t.Fatal("cancelling created a planet source file", err)
	}
	w.app.CommandStorage().Undo()
	if !reflect.DeepEqual(original.State, before) {
		t.Fatal("returning from cancellation lost the original undo history")
	}
}
