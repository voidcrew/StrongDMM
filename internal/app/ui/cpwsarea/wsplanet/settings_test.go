package wsplanet

import (
	"sdmm/internal/planet"
	"testing"
)

func testPlanetSettingsScreens(t *testing.T, w *Workspace, capture func(string), width, height *int) {
	t.Helper()
	before := planet.Clone(w.project.State)
	defer func() {
		w.project.State = before
		w.lighting = false
		w.picking, w.pickingRiver, w.pickingEnvironment = false, false, false
	}()
	w.mode, w.settingsTab, w.narrowEditor = 3, 1, true
	w.cave = false
	capture("settings-rivers")
	w.beginRiverPicker()
	capture("settings-river-picker")
	if w.thumbnail(w.project.State.Definition.Rivers.Turf) == nil {
		t.Fatal("missing river thumbnail")
	}
	w.picking = false
	w.settingsTab, w.lighting = 2, true
	capture("settings-environment")
	if w.scene == nil || w.scene.Lighting == nil {
		t.Fatal("daylight preview missing")
	}
	w.project.State.Definition.Environment.LightColor = "#A4D8FF"
	w.project.State.Definition.Environment.LightAlpha = 160
	capture("settings-environment-edited")
	w.settingsTab = 3
	capture("settings-ruins")
	w.project.State.Definition.Ruins.Templates = []string{}
	capture("settings-no-ruins")
	*width, *height = 1000, 760
	w.settingsTab = 2
	capture("compact-environment")
	w.settingsTab = 1
	capture("compact-rivers")
	w.settingsTab = 3
	capture("compact-ruins")
	*width, *height = 1600, 1000
}
