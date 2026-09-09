package wsplanet

import (
	"fmt"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/workshop"
	"sdmm/internal/app/window"
	"sdmm/internal/imguiext/style"
)

func (w *Workspace) beginDeleteBiome() {
	if !w.commitName() {
		return
	}
	if _, ok := w.project.State.Biomes[w.selected]; !ok {
		return
	}
	w.record()
	w.deleting, w.narrowEditor = true, true
	w.picking, w.review = false, false
	w.deleteReplacement = ""
	w.message = ""
	w.mode, w.fit = 1, true
}

func (w *Workspace) deleteBiome() {
	b := w.project.State.Biomes[w.selected]
	w.record()
	if err := w.project.DeleteBiome(w.selected, w.deleteReplacement); err != nil {
		w.message = err.Error()
		return
	}
	w.recordNamed("Delete biome")
	next := w.deleteReplacement
	if next == "" {
		next = w.project.State.UsedBiomes()[0]
	}
	w.selectBiome(next)
	if w.climateView.brush == b.Path {
		w.climateView.brush = next
	}
	w.renderKey = ""
	w.mode, w.fit = 0, true
	w.message = fmt.Sprintf("%s deleted from this planet. Undo restores it.", b.Name)
}

func (w *Workspace) deleteBiomeForm() {
	b := w.project.State.Biomes[w.selected]
	workshop.Title("Delete biome")
	workshop.Section(b.Name, style.Teal)
	surface, caves := w.project.BiomeUse(w.selected)
	if surface+caves == 0 {
		workshop.Muted("This biome is not assigned to any climate cells. Deleting it removes it from this planet.")
	} else {
		workshop.Muted(fmt.Sprintf("Used in %d surface and %d cave climate cells. Choose the biome that will take its place in all of them.", surface, caves))
		if caves > 0 {
			workshop.Muted("The replacement must include cave walls.")
		}
		workshop.Gap()
		workshop.Section("REPLACE WITH", style.Teal)
		imgui.BeginChildV("delete-biome-choices", imgui.Vec2{Y: min(300*window.PointSize(), max(130*window.PointSize(), imgui.ContentRegionAvail().Y-165*window.PointSize()))}, true, 0)
		for _, choice := range w.project.BiomeChoices(caves > 0) {
			if choice.Path == w.selected {
				continue
			}
			detail := "Surface"
			if choice.Cave {
				detail = "Cave walls included"
			}
			pos := imgui.CursorScreenPos()
			if workshop.Row(choice.Path, choice.Name, detail, "", choice.Path == w.deleteReplacement, style.Teal, 38) {
				w.deleteReplacement = choice.Path
			}
			w.sprite(w.biomeGround(choice), imgui.Vec2{X: pos.X + 9*window.PointSize(), Y: pos.Y + 12*window.PointSize()}, 32*window.PointSize())
		}
		imgui.EndChild()
	}
	workshop.Gap()
	problem := w.project.CheckDeleteBiome(w.selected, w.deleteReplacement)
	if problem != nil {
		workshop.Muted(problem.Error())
	}
	label := "Delete biome"
	if surface+caves > 0 {
		label = "Replace & delete biome"
	}
	imgui.BeginDisabledV(problem != nil)
	if workshop.Button(label, true) {
		w.deleteBiome()
	}
	imgui.EndDisabled()
	if workshop.Button("Cancel", false) {
		w.deleting = false
	}
	workshop.Muted("You can undo this change before or after saving.")
}
