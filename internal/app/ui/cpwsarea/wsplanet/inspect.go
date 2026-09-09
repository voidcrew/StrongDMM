package wsplanet

import (
	"math"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/render/bucket/level/chunk/unit"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/mappreview"
	"sdmm/internal/planet"
)

// Units arrive in drawing order. Keep the last opaque sprite pixel under the
// pointer, just as the map editor does, including pixel offsets and tall sprites.
func (w *Workspace) ProcessUnit(u unit.Unit) bool {
	if !mappreview.Visible(u.Instance().Prefab()) {
		return false
	}
	if w.hoverActive && spriteHit(u, w.mouseWorld) {
		w.hovered = u.Instance()
	}
	return true
}

func spriteHit(u unit.Unit, point imgui.Vec2) bool {
	b, sprite := u.ViewBounds(), u.Sprite()
	if sprite == nil || u.A() == 0 || point.X < b.X1 || point.Y < b.Y1 || point.X >= b.X2 || point.Y >= b.Y2 {
		return false
	}
	x := int(point.X-b.X1) + sprite.X1
	y := sprite.IconHeight() - 1 - int(point.Y-b.Y1) + sprite.Y1
	_, _, _, alpha := sprite.Image().At(x, y).RGBA()
	return alpha != 0
}

type previewChoice struct {
	cell               planet.Cell
	path, field, group string
}

func (w *Workspace) previewTarget() (previewChoice, bool) {
	if w.preview == nil {
		return previewChoice{}, false
	}
	x, y := int(math.Floor(float64(w.mouseWorld.X/32))), int(math.Floor(float64(w.mouseWorld.Y/32)))
	if w.hovered != nil {
		// An overhanging sprite belongs to its origin biome, not the ground
		// beneath the mouse. Display-only spawner children use that same origin.
		x, y = w.hovered.Coord().X-1, w.hovered.Coord().Y-1
	}
	if x < 0 || y < 0 || x >= w.preview.Map.MaxX || y >= w.preview.Map.MaxY {
		return previewChoice{}, false
	}
	cell := w.preview.Cells[y*w.preview.Map.MaxX+x]
	target := previewChoice{cell: cell, path: cell.Turf, field: "open_turf_types"}
	if cell.River {
		target.field, target.group = "river", "Rivers"
		return target, true
	}
	if cell.Closed {
		target.field = "closed_turf_types"
	}
	if w.hovered != nil && !dm.IsPath(w.hovered.Prefab().Path(), "/turf") && cell.Spawn != "" {
		target.path, target.field = cell.Spawn, cell.SpawnField
	}
	for _, table := range w.project.State.Biomes[cell.Biome].Tables {
		if table.Field == target.field {
			target.group = table.Name
			return target, true
		}
	}
	return previewChoice{}, false
}

func (w *Workspace) inspectPreview(target previewChoice) {
	if target.field == "river" {
		if w.commitName() {
			w.picking, w.deleting = false, false
			w.mode, w.narrowEditor = 3, true
			w.settingsTab = 1
		}
		return
	}
	// The displayed frame can lag a live edit briefly. Do not select a biome
	// that has already been replaced or deleted while the preview catches up.
	visible := false
	for _, path := range w.project.State.VisibleBiomes() {
		visible = visible || path == target.cell.Biome
	}
	if !visible || !w.selectBiome(target.cell.Biome) {
		return
	}
	for i, table := range w.project.State.Biomes[w.selected].Tables {
		if table.Field != target.field {
			continue
		}
		for _, entry := range table.Entries {
			if entry.Path == target.path {
				w.table = i
				w.selectedEntry, w.revealEntry = entry.Path, true
				w.narrowEditor = true
				if w.mode == 3 {
					w.mode = 0
				}
				return
			}
		}
	}
}
