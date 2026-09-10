package wsplanet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sdmm/internal/app/command"
	"sdmm/internal/app/ui/cpwsarea/workspace"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/planet"
	"sdmm/internal/planet/planettest"
)

func newTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	dme, err := dmenv.New(planettest.Write(t))
	if err != nil {
		t.Fatal(err)
	}
	w := build(&testApp{dme, command.NewStorage()}, nil)
	if w.project == nil {
		t.Fatal(w.message)
	}
	return w
}

func reopen(t *testing.T, w *Workspace, path string) *planet.Project {
	t.Helper()
	dme, err := dmenv.New(w.catalog.Dme.RootFile)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := planet.Discover(dme)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range catalog.Planets {
		if d.Path == path {
			p, err := planet.Open(catalog, d)
			if err != nil {
				t.Fatal(err)
			}
			return p
		}
	}
	t.Fatal("planet is not registered", path)
	return nil
}

// File > Save and Ctrl+S reach Workspace.Save. The visible planet must be
// written even when another open draft cannot be saved yet.
func TestFileSaveWritesVisiblePlanet(t *testing.T) {
	w := newTestWorkspace(t)
	ws := workspace.New(w)
	w.switchPlanet("/datum/planet/other")
	if w.project.State.Definition.Path != "/datum/planet/other" {
		t.Fatal(w.message)
	}
	w.biomeName, w.nameDirty = "", true // An unfinished biome name blocks saving.
	w.switchPlanet("/datum/planet/test")
	w.project.State.Seeds.Heat = 4242
	w.record()
	if !ws.Save() {
		t.Fatal("visible planet was not saved:", w.message)
	}
	if w.project.State.Definition.Path != "/datum/planet/test" || w.currentModified() || !w.IsModified() {
		t.Fatalf("save changed the visible planet or lost the other draft: %s modified=%v", w.project.State.Definition.Path, w.currentModified())
	}
	if p := reopen(t, w, "/datum/planet/test"); p.State.Seeds.Heat != 4242 {
		t.Fatal("saved planet does not reopen with its edits", p.State.Seeds.Heat)
	}
	dme, _ := os.ReadFile(w.catalog.Dme.RootFile)
	if !strings.Contains(string(dme), `#include "voidcrew\mapping\planet_projects\test.dm"`) {
		t.Fatalf("registration missing or wrongly slashed:\n%s", dme)
	}
	if ws.SaveAll() {
		t.Fatal("Save All ignored an invalid draft")
	}
	if w.project.State.Definition.Path != "/datum/planet/other" || !w.nameDirty || w.message == "" {
		t.Fatal("failed draft is not shown with its reason", w.project.State.Definition.Path, w.message)
	}
	w.biomeName = "Fixed name"
	w.switchPlanet("/datum/planet/test")
	if !ws.SaveAll() {
		t.Fatal("Save All failed after the fix:", w.message)
	}
	if w.project.State.Definition.Path != "/datum/planet/test" || w.IsModified() {
		t.Fatal("Save All left drafts dirty or moved away from the visible planet", w.project.State.Definition.Path, w.IsModified())
	}
	if p := reopen(t, w, "/datum/planet/other"); p.Modified() {
		t.Fatal("other draft did not save cleanly")
	}
}

// Building a planet and pressing Ctrl+S must create it, and an invalid form
// must keep its contents while explaining the failure.
func TestFileSaveCompletesCreationForm(t *testing.T) {
	w := newTestWorkspace(t)
	ws := workspace.New(w)
	w.startCreation()
	for i, d := range w.catalog.Planets {
		if d.Path == "/datum/planet/test" {
			w.base = i // The starting environment with an overmap registration.
		}
	}
	w.newName, w.blank = "Glasswood", true
	if !w.IsModified() {
		t.Fatal("a named creation form is not treated as unsaved work")
	}
	if !ws.Save() {
		t.Fatal("creation form was not saved:", w.message)
	}
	if w.creating || w.project.State.Definition.Path != "/datum/planet/workshop_glasswood" || w.project.UnsavedNew() {
		t.Fatal("planet was not created and saved", w.creating, w.message)
	}
	if filepath.Base(w.project.GeneratedPath()) != "glasswood.dm" {
		t.Fatal("generated file keeps the internal prefix", w.project.GeneratedPath())
	}
	if _, err := os.Stat(w.project.GeneratedPath()); err != nil {
		t.Fatal(err)
	}
	if d := reopen(t, w, "/datum/planet/workshop_glasswood"); d.State.Definition.Name != "Glasswood" {
		t.Fatal("new planet does not reopen", d.State.Definition.Name)
	}
	w.startCreation()
	w.newName = "glasswood"
	if ws.Save() || !w.creating || w.newName != "glasswood" || w.message == "" {
		t.Fatal("duplicate creation form was accepted or cleared", w.creating, w.newName, w.message)
	}
	w.newName = ""
	if !ws.Save() || !w.creating {
		t.Fatal("an empty form should not block saving", w.message)
	}
}
