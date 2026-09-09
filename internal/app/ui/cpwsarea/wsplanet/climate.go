package wsplanet

import (
	"fmt"
	"image"
	"math"

	"github.com/SpaiR/imgui-go"
	"github.com/go-gl/gl/v3.3-core/gl"
	"sdmm/internal/app/ui/workshop"
	"sdmm/internal/app/window"
	"sdmm/internal/imguiext/style"
	"sdmm/internal/planet"
	"sdmm/internal/platform"
)

type climateView struct {
	brush                         string
	painting, stroke, layer       bool
	focused, hoverValid, mapHover bool
	hover                         planet.ClimateCell
	lens                          int
	before                        map[planet.ClimateCell]string
	counts                        map[planet.ClimateCell]int
	lastPaint                     planet.ClimateCell
	lastPaintValid                bool
	overlayKey                    climateOverlayKey
	// Screen bounds also let native acceptance exercise actual pointer strokes.
	gridMin, cellSize imgui.Vec2
}

type climateOverlayKey struct {
	preview *planet.Preview
	cell    planet.ClimateCell
	lens    int
	focused bool
	changes uint64
}

func (w *Workspace) climateGrid(caves bool) [][]string {
	if caves {
		return w.project.State.Definition.Caves
	}
	return w.project.State.Definition.Surface
}

func (w *Workspace) climatePath(cell planet.ClimateCell) string {
	grid := w.climateGrid(cell.Caves)
	if cell.Row < 0 || cell.Row >= len(grid) || cell.Col < 0 || cell.Col >= len(grid[cell.Row]) {
		return ""
	}
	return grid[cell.Row][cell.Col]
}

func climateLabel(cell planet.ClimateCell) string {
	names := planet.HeatNames
	if cell.Caves {
		names = planet.CaveNames
	}
	return names[cell.Row] + " / " + planet.MoistureNames[cell.Col]
}

func (w *Workspace) climateBrush() planet.Biome {
	b, ok := w.project.State.Biomes[w.climateView.brush]
	if !ok {
		for _, choice := range w.project.BiomeChoices(false) {
			if choice.Path == w.climateView.brush {
				return choice
			}
		}
	}
	return b
}

func (w *Workspace) beginClimateStroke() bool {
	if w.climateView.stroke {
		return true
	}
	if !w.commitName() {
		return false
	}
	w.record()
	v := &w.climateView
	v.before = map[planet.ClimateCell]string{}
	for _, caves := range []bool{false, true} {
		for row, cells := range w.climateGrid(caves) {
			for col, path := range cells {
				v.before[planet.ClimateCell{Row: row, Col: col, Caves: caves}] = path
			}
		}
	}
	v.stroke = true
	v.lastPaintValid = false
	return true
}

func (w *Workspace) paintClimate(cell planet.ClimateCell) {
	b := w.climateBrush()
	if b.Path == "" || (cell.Caves && !b.Cave) || w.climatePath(cell) == "" {
		return
	}
	if !w.beginClimateStroke() {
		return
	}
	w.project.State.Biomes[b.Path] = b
	w.climateGrid(cell.Caves)[cell.Row][cell.Col] = b.Path
	w.row, w.col = cell.Row, cell.Col
	w.climateView.focused = true
}

func (w *Workspace) dragClimate(cell planet.ClimateCell) {
	v := &w.climateView
	if v.stroke && v.lastPaintValid && v.lastPaint.Caves == cell.Caves {
		start := v.lastPaint
		dx, dy := cell.Col-start.Col, cell.Row-start.Row
		steps := max(int(math.Abs(float64(dx))), int(math.Abs(float64(dy))))
		for i := 1; i < steps; i++ {
			w.paintClimate(planet.ClimateCell{Row: start.Row + int(math.Round(float64(dy*i)/float64(steps))), Col: start.Col + int(math.Round(float64(dx*i)/float64(steps))), Caves: cell.Caves})
		}
	}
	w.paintClimate(cell)
	v.lastPaint, v.lastPaintValid = cell, v.stroke
}

func (w *Workspace) finishClimateStroke() {
	if w.climateView.stroke {
		w.climateView.stroke = false
		w.recordNamed("Paint climate")
	}
}

func (w *Workspace) focusClimate(cell planet.ClimateCell) {
	path := w.climatePath(cell)
	if path == "" || !w.selectBiome(path) {
		return
	}
	w.row, w.col = cell.Row, cell.Col
	w.climateView.layer, w.climateView.focused = cell.Caves, true
}

func (w *Workspace) climateCountChanges() int {
	count := 0
	for cell, path := range w.climateView.before {
		if w.climatePath(cell) != path {
			count += w.climateView.counts[cell]
		}
	}
	return count
}

func (w *Workspace) climate() {
	v := &w.climateView
	if v.brush == "" {
		v.brush = w.selected
		v.layer = w.cave
	}
	v.counts = map[planet.ClimateCell]int{}
	if w.preview != nil {
		for _, cell := range w.preview.Cells {
			v.counts[cell.Climate]++
		}
	}
	workshop.Title("Where biomes grow")
	workshop.Muted("Explore a climate cell to find its terrain. Choose a biome, then paint across the grid.")
	s := window.PointSize()
	pos, available := imgui.CursorPos(), imgui.ContentRegionAvail()
	gridPos, gridSize, mapPos, mapSize := pos, available, pos, available
	if available.X >= 700*s {
		gridSize.X = min(450*s, available.X*.49)
		mapPos.X += gridSize.X + 12*s
		mapSize.X -= gridSize.X + 12*s
	} else {
		// Each pane has its own space: scrolling the rules never hides the map.
		mapSize.Y = max(230*s, available.Y*.52)
		gridPos.Y += mapSize.Y + 8*s
		gridSize.Y = max(80*s, available.Y-mapSize.Y-8*s)
	}
	imgui.SetCursorPos(gridPos)
	workshop.Panel("climate-rules", gridSize, false)
	w.climateControls()
	workshop.EndPanel()
	imgui.SetCursorPos(mapPos)
	imgui.BeginChildV("climate-preview", mapSize, false, imgui.WindowFlagsNoScrollbar|imgui.WindowFlagsNoScrollWithMouse)
	w.climateMap()
	imgui.EndChild()
	imgui.SetCursorPos(imgui.Vec2{X: pos.X, Y: pos.Y + available.Y})
}

func climateToggle(label string, active bool) bool {
	if active {
		imgui.PushStyleColor(imgui.StyleColorButton, style.RGB(0x28565b))
	}
	clicked := imgui.Button(label)
	if active {
		imgui.PopStyleColor()
	}
	return clicked
}

func (w *Workspace) climateControls() {
	v := &w.climateView
	for i, name := range []string{"Surface", "Caves"} {
		if i > 0 {
			imgui.SameLine()
		}
		if climateToggle(name, v.layer == (i == 1)) {
			v.layer, w.cave = i == 1, i == 1
			v.focused, v.hoverValid, v.mapHover = false, false, false
			w.row, w.col = 0, 0
		}
	}
	imgui.SameLine()
	if climateToggle("Inspect", !v.painting) {
		v.painting = false
	}
	imgui.SameLine()
	if climateToggle("Paint", v.painting) {
		v.painting = true
	}
	b := w.climateBrush()
	label := b.Name
	if b.Path == "" {
		label = "Choose a biome"
	}
	if combo("Biome brush", label) {
		for _, choice := range w.project.BiomeChoices(v.layer) {
			pos := imgui.CursorScreenPos()
			if imgui.SelectableV("    "+choice.Name+"##brush-"+choice.Path, v.brush == choice.Path, 0, imgui.Vec2{Y: 26 * window.PointSize()}) {
				if w.commitName() {
					v.brush, v.painting = choice.Path, true
				}
			}
			w.sprite(w.biomeGround(choice), pos, 22*window.PointSize())
		}
		imgui.EndCombo()
	}
	grid := w.climateGrid(v.layer)
	if len(grid) == 0 {
		workshop.Muted("This planet has no rules for this layer.")
		if workshop.Button("Add this layer", true) {
			choices := w.project.BiomeChoices(v.layer)
			if len(choices) > 0 {
				choice := choices[0]
				w.project.State.Biomes[choice.Path] = choice
				rows := 6
				if v.layer {
					rows = 4
				}
				grid = make([][]string, rows)
				for row := range grid {
					grid[row] = []string{choice.Path, choice.Path, choice.Path, choice.Path, choice.Path}
				}
				if v.layer {
					w.project.State.Definition.Caves = grid
				} else {
					w.project.State.Definition.Surface = grid
				}
				v.brush = choice.Path
			}
		}
		v.hoverValid = false
		return
	}
	w.row, w.col = min(w.row, len(grid)-1), min(w.col, 4)
	if v.painting && b.Path != "" && v.layer && !b.Cave {
		workshop.Muted("Choose a biome brush for this layer.")
	} else {
		workshop.Muted("Drier to wetter across; colder to hotter down.")
	}
	w.climateTiles()
	cell := planet.ClimateCell{Row: w.row, Col: w.col, Caves: v.layer}
	path := w.climatePath(cell)
	workshop.Section(climateLabel(cell), style.Teal)
	workshop.Wrapped(w.project.State.Biomes[path].Name)
	count, total := v.counts[cell], 0
	if w.preview != nil {
		total = len(w.preview.Cells)
	}
	if count == 0 {
		workshop.Muted("No matching tiles in this preview.")
	} else {
		workshop.Muted(fmt.Sprintf("%s of this preview (%d tiles)", climatePercent(count, total), count))
	}
	if imgui.Button("Use as brush") {
		v.brush, v.painting = path, true
	}
	imgui.SameLine()
	if imgui.Button("Edit biome") && w.selectBiome(path) {
		w.mode, w.narrowEditor, w.fit = 1, true, true
	}
	if imgui.Button("Paint row") {
		for col := range grid[cell.Row] {
			w.paintClimate(planet.ClimateCell{Row: cell.Row, Col: col, Caves: v.layer})
		}
		w.finishClimateStroke()
	}
	imgui.SameLine()
	if imgui.Button("Paint column") {
		for row := range grid {
			w.paintClimate(planet.ClimateCell{Row: row, Col: cell.Col, Caves: v.layer})
		}
		w.finishClimateStroke()
	}
}

func climatePercent(count, total int) string {
	if total == 0 || count == 0 {
		return "0%"
	}
	value := float64(count) * 100 / float64(total)
	if value < .1 {
		return "<0.1%"
	}
	return fmt.Sprintf("%.1f%%", value)
}

func (w *Workspace) climateTiles() {
	v, s := &w.climateView, window.PointSize()
	grid, names := w.climateGrid(v.layer), planet.HeatNames
	if v.layer {
		names = planet.CaveNames
	}
	start := imgui.CursorScreenPos()
	labelWidth, gap := 73*s, 4*s
	width := (imgui.ContentRegionAvail().X - labelWidth) / 5
	height := 71 * s
	v.gridMin = imgui.Vec2{X: start.X + labelWidth, Y: start.Y + 24*s}
	v.cellSize = imgui.Vec2{X: width, Y: height}
	draw := imgui.WindowDrawList()
	text, muted := imgui.PackedColorFromVec4(style.Text), imgui.PackedColorFromVec4(style.Muted)
	for col, label := range planet.MoistureNames {
		draw.AddText(imgui.Vec2{X: v.gridMin.X + float32(col)*width, Y: start.Y}, muted, workshop.Ellipsis(label, width-gap))
	}
	mapHover, mapCell := v.mapHover, v.hover
	v.hoverValid = false
	total := 0
	if w.preview != nil {
		total = len(w.preview.Cells)
	}
	for row, cells := range grid {
		y := v.gridMin.Y + float32(row)*height
		draw.AddText(imgui.Vec2{X: start.X, Y: y + 22*s}, muted, names[row])
		for col, path := range cells {
			cell := planet.ClimateCell{Row: row, Col: col, Caves: v.layer}
			pos := imgui.Vec2{X: v.gridMin.X + float32(col)*width, Y: y}
			end := imgui.Vec2{X: pos.X + width - gap, Y: pos.Y + height - gap}
			imgui.SetCursorScreenPos(pos)
			imgui.PushIDInt(row*5 + col)
			imgui.InvisibleButton("climate-cell", end.Minus(pos))
			// Allow a stroke to cross items while its first cell holds ImGui's active ID.
			hover := imgui.IsItemHoveredV(imgui.HoveredFlagsAllowWhenBlockedByActiveItem)
			if hover {
				v.hover, v.hoverValid = cell, true
				if imgui.IsMouseClicked(imgui.MouseButtonLeft) {
					if v.painting {
						w.dragClimate(cell)
					} else {
						w.focusClimate(cell)
					}
				} else if v.painting && v.stroke && imgui.IsMouseDown(imgui.MouseButtonLeft) {
					w.dragClimate(cell)
				}
				if imgui.IsMouseClicked(imgui.MouseButtonRight) {
					v.brush, v.painting = path, true
				}
			}
			fill := style.Raised
			if v.counts[cell] == 0 {
				fill = style.Background
			}
			draw.AddRectFilledV(pos, end, imgui.PackedColorFromVec4(fill), 4*s, 0)
			b := w.project.State.Biomes[w.climatePath(cell)]
			w.sprite(w.biomeGround(b), imgui.Vec2{X: pos.X + 5*s, Y: pos.Y + 4*s}, 26*s)
			draw.AddText(imgui.Vec2{X: pos.X + 5*s, Y: pos.Y + 32*s}, text, workshop.Ellipsis(b.Name, width-gap-9*s))
			draw.AddText(imgui.Vec2{X: pos.X + 5*s, Y: pos.Y + 49*s}, muted, climatePercent(v.counts[cell], total))
			if hover || (mapHover && mapCell == cell) || (v.focused && w.row == row && w.col == col) {
				draw.AddRectV(pos, end, imgui.PackedColorFromVec4(style.Teal), 4*s, 0, 2*s)
			}
			if hover {
				imgui.BeginTooltip()
				imgui.Text(climateLabel(cell) + " -> " + b.Name)
				imgui.Text(fmt.Sprintf("%d tiles / %s of this preview", v.counts[cell], climatePercent(v.counts[cell], total)))
				imgui.EndTooltip()
			}
			imgui.PopID()
		}
	}
	imgui.SetCursorScreenPos(imgui.Vec2{X: start.X, Y: v.gridMin.Y + float32(len(grid))*height + 4*s})
}

func (w *Workspace) climateMap() {
	v := &w.climateView
	imgui.SetNextItemWidth(max(100*window.PointSize(), imgui.ContentRegionAvail().X-120*window.PointSize()))
	labels := []string{"Terrain", "Heat", "Moisture", "Last stroke"}
	if imgui.BeginCombo("##climate-lens", labels[v.lens]) {
		for i, label := range labels {
			if imgui.Selectable(label) {
				v.lens = i
			}
		}
		imgui.EndCombo()
	}
	imgui.SameLine()
	if imgui.Button("Fit") {
		w.fit = true
	}
	imgui.SameLine()
	if imgui.Button("Clear") {
		v.focused, v.hoverValid = false, false
	}
	if w.preview == nil {
		workshop.Muted(w.message)
		return
	}
	if v.lens == 1 {
		workshop.Muted("Blue: colder  /  Gold: hotter")
	} else if v.lens == 2 {
		workshop.Muted("Sand: drier  /  Blue: wetter")
	} else if v.lens == 3 {
		workshop.Muted(fmt.Sprintf("%d tiles changed by the last stroke", w.climateCountChanges()))
	} else {
		workshop.Muted("Same seed while editing. Scroll to zoom; middle-drag to pan.")
	}
	w.previewCanvas(imgui.Vec2{Y: -64 * window.PointSize()})
	cell, focused := v.hover, v.hoverValid
	if !focused && v.focused {
		cell, focused = planet.ClimateCell{Row: w.row, Col: w.col, Caves: v.layer}, true
	}
	if focused && w.climatePath(cell) != "" {
		workshop.Wrapped(climateLabel(cell) + " -> " + w.project.State.Biomes[w.climatePath(cell)].Name)
		count, total := v.counts[cell], len(w.preview.Cells)
		b := w.climateBrush()
		if v.painting && v.hoverValid && !v.mapHover && b.Path != "" && (!cell.Caves || b.Cave) {
			change := count
			if w.climatePath(cell) == b.Path {
				change = 0
			}
			workshop.Muted(fmt.Sprintf("Paint %s: %d tiles would change (%s).", b.Name, change, climatePercent(change, total)))
		} else {
			workshop.Muted(fmt.Sprintf("%d matching tiles / %s of this preview", count, climatePercent(count, total)))
		}
	} else {
		workshop.Muted("Hover a grid cell to reveal its terrain. Click the map to select its rule.")
	}
}

func (w *Workspace) climateMapInput() {
	v := &w.climateView
	v.mapHover = false
	if !w.control.Active() || w.control.Moving() {
		return
	}
	x, y := int(math.Floor(float64(w.mouseWorld.X/32))), int(math.Floor(float64(w.mouseWorld.Y/32)))
	if x < 0 || y < 0 || x >= w.preview.Map.MaxX || y >= w.preview.Map.MaxY {
		return
	}
	cell := w.preview.Cells[y*w.preview.Map.MaxX+x]
	v.hover, v.hoverValid, v.mapHover = cell.Climate, true, true
	if imgui.IsMouseClicked(imgui.MouseButtonLeft) {
		w.focusClimate(cell.Climate)
	}
	imgui.BeginTooltip()
	imgui.Text(climateLabel(cell.Climate) + " -> " + w.project.State.Biomes[w.climatePath(cell.Climate)].Name)
	imgui.Text(fmt.Sprintf("Heat %.0f%% / Moisture %.0f%%", cell.Heat*100, cell.Moisture*100))
	if cell.Climate.Caves && !w.cave {
		imgui.Text("Mountain terrain uses cave climate.")
	}
	if cell.River {
		imgui.Text("A river replaces the ground here.")
	}
	imgui.EndTooltip()
}

func (w *Workspace) climateOverlay() {
	v := &w.climateView
	cell, focused := v.hover, v.hoverValid
	if !focused && v.focused {
		cell, focused = planet.ClimateCell{Row: w.row, Col: w.col, Caves: v.layer}, true
	}
	if !focused && v.lens == 0 {
		return
	}
	draw, camera := imgui.WindowDrawList(), w.canvas.Render().Camera
	lo, hi := w.control.PosMin(), w.control.PosMax()
	draw.PushClipRectV(lo, hi, true)
	defer draw.PopClipRect()
	width, height := w.preview.Map.MaxX, w.preview.Map.MaxY
	key := climateOverlayKey{preview: w.preview, cell: cell, lens: v.lens, focused: focused}
	for address, old := range v.before {
		if w.climatePath(address) != old {
			bit := address.Row*5 + address.Col
			if address.Caves {
				bit += 30
			}
			key.changes |= 1 << bit
		}
	}
	color := func(c planet.Cell) imgui.Vec4 {
		if v.lens == 3 {
			old, ok := v.before[c.Climate]
			if ok && old != w.climatePath(c.Climate) {
				return imgui.Vec4{X: .95, Y: .75, Z: .28, W: .55}
			}
			return imgui.Vec4{W: .73}
		}
		if v.lens == 1 || v.lens == 2 {
			a, b, amount := style.RGB(0x447bb0), style.RGB(0xf0b356), float32(c.Heat)
			if v.lens == 2 {
				a, b, amount = style.RGB(0xc3a171), style.RGB(0x367daf), float32(c.Moisture)
			}
			alpha := float32(.85)
			if focused && c.Climate != cell {
				alpha = .4
			}
			return imgui.Vec4{X: a.X + (b.X-a.X)*amount, Y: a.Y + (b.Y-a.Y)*amount, Z: a.Z + (b.Z-a.Z)*amount, W: alpha}
		}
		if c.Climate == cell {
			return imgui.Vec4{X: .3, Y: .9, Z: .8, W: .2}
		}
		return imgui.Vec4{W: .72}
	}
	// One texel per tile keeps large previews below ImGui's 16-bit vertex limit.
	// Camera movement only moves the quad; it does not rebuild the overlay.
	if key != v.overlayKey || w.climateTexture == 0 {
		pixels := image.NewNRGBA(image.Rect(0, 0, width, height))
		for i, c := range w.preview.Cells {
			shade := color(c)
			pixels.Pix[i*4], pixels.Pix[i*4+1], pixels.Pix[i*4+2], pixels.Pix[i*4+3] = byte(shade.X*255), byte(shade.Y*255), byte(shade.Z*255), byte(shade.W*255)
		}
		gl.DeleteTextures(1, &w.climateTexture)
		w.climateTexture = platform.CreateTexture(pixels)
		v.overlayKey = key
	}
	topLeft := imgui.Vec2{X: lo.X + camera.ShiftX*camera.Scale, Y: hi.Y - (float32(height*32)+camera.ShiftY)*camera.Scale}
	bottomRight := imgui.Vec2{X: lo.X + (float32(width*32)+camera.ShiftX)*camera.Scale, Y: hi.Y - camera.ShiftY*camera.Scale}
	draw.AddImageV(imgui.TextureID(w.climateTexture), topLeft, bottomRight, imgui.Vec2{Y: 1}, imgui.Vec2{X: 1}, style.ColorWhitePacked)
}
