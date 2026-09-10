package wsplanet

import (
	"testing"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/planet"
)

// Dragging the second biome row above the first reorders the list with a real
// pointer and records a single undo step once the button is released.
func testBiomeReorder(t *testing.T, w *Workspace, io imgui.IO, render func(), capture func(string)) {
	t.Helper()
	saved := planet.Clone(w.project.State)
	defer func() {
		io.SetMouseButtonDown(0, false)
		io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
		w.project.State = saved
		w.bind(w.project)
	}()
	w.mode = 0
	render()
	render()
	visible := w.project.State.VisibleBiomes()
	if len(visible) < 2 {
		t.Fatal("reorder needs two biomes", visible)
	}
	first, second := visible[0], visible[1]
	from, to := w.biomeRows[second], w.biomeRows[first]
	if from.Y <= to.Y || to.Y <= 0 {
		t.Fatal("biome rows were not laid out", from, to)
	}
	io.SetMousePosition(from)
	render()
	io.SetMouseButtonDown(0, true)
	render()
	render()
	io.SetMousePosition(to)
	for i := 0; i < 4; i++ {
		render()
	}
	if got := w.project.State.VisibleBiomes(); got[0] != second || got[1] != first {
		t.Fatal("drag did not reorder the biomes", got)
	}
	if !w.reordering || undoLabel(w) == "Reorder biomes" {
		t.Fatal("reorder was recorded before the drag ended")
	}
	io.SetMouseButtonDown(0, false)
	render()
	render()
	capture("biome-reordered")
	if w.reordering || undoLabel(w) != "Reorder biomes" {
		t.Fatalf("drag end did not record one reorder step: reordering=%v, undo=%q", w.reordering, undoLabel(w))
	}
	w.app.CommandStorage().Undo()
	if got := w.project.State.VisibleBiomes(); got[0] != first || got[1] != second {
		t.Fatal("undo did not restore the order", got)
	}
	w.app.CommandStorage().Redo()
	if got := w.project.State.VisibleBiomes(); got[0] != second || got[1] != first {
		t.Fatal("redo did not reapply the order", got)
	}
}

func undoLabel(w *Workspace) string {
	if !w.app.CommandStorage().HasUndo() {
		return ""
	}
	return w.app.CommandStorage().UndoName()
}
