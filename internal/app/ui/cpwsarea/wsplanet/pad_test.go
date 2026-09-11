package wsplanet

import (
	"testing"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/planet"
)

// Dragging the climate handle toward wet and hot (right and down, like the
// grid) reshapes every band scale in one undo step, and releasing it leaves the
// handle where it was dropped.
func testClimatePad(t *testing.T, w *Workspace, io imgui.IO, render func(), capture func(string)) {
	t.Helper()
	saved := planet.Clone(w.project.State)
	defer func() {
		io.SetMouseButtonDown(0, false)
		io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
		w.project.State = saved
		w.bind(w.project)
	}()
	if !w.catalog.ClimateShares {
		t.Fatal("the project's generator has no climate shares")
	}
	w.mode, w.settingsTab, w.narrowEditor = 3, 0, true
	render()
	render()
	if w.padSize <= 0 {
		t.Fatal("climate pad was not laid out")
	}
	// x is the moisture bias (wetter to the right); y is the heat bias (hotter
	// downward), the same orientation as the climate grid's columns and rows.
	at := func(x, y float32) imgui.Vec2 {
		return imgui.Vec2{X: w.padMin.X + (x+1)/2*w.padSize, Y: w.padMin.Y + (y+1)/2*w.padSize}
	}
	io.SetMousePosition(at(0, 0))
	render()
	io.SetMouseButtonDown(0, true)
	render()
	io.SetMousePosition(at(.5, .75))
	render()
	render()
	g := w.project.State.Definition.Settings
	if g.Heat[5] <= g.Heat[0] || g.CaveHeat[3] <= g.CaveHeat[0] || g.Moisture[4] <= g.Moisture[0] {
		t.Fatal("dragging the handle did not reshape the bands", g)
	}
	if wetBias := planet.ShareBias(g.Moisture[:], planet.DefaultMoistureShares[:]); wetBias < .45 || wetBias > .55 {
		t.Fatal("the pad's horizontal axis is not moisture", wetBias)
	}
	io.SetMouseButtonDown(0, false)
	render()
	render()
	capture("climate-pad")
	if w.app.CommandStorage().UndoName() != "Edit planet" {
		t.Fatalf("handle drag did not record one step: %q", w.app.CommandStorage().UndoName())
	}
	heat := g.HeatShares()
	if bias := planet.ShareBias(heat[:], planet.DefaultHeatShares[:]); bias < .7 || bias > .8 {
		t.Fatal("handle does not read back where it was dropped", bias)
	}
	w.app.CommandStorage().Undo()
	if w.project.State.Definition.Settings.HeatShares() != planet.DefaultHeatShares {
		t.Fatal("undo did not restore the default balance")
	}
}
