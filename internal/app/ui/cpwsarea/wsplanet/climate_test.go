package wsplanet

import (
	"reflect"
	"testing"
	"time"

	"github.com/SpaiR/imgui-go"
	"github.com/go-gl/gl/v3.3-core/gl"
	"sdmm/internal/planet"
)

// Exercise native hover, click, fast drag, release outside the grid and undo.
// These interactions depend on ImGui active IDs and the real camera transform.
func testClimatePainting(t *testing.T, w *Workspace, io imgui.IO, render func(), capture func(string), width, height *int) {
	t.Helper()
	saved, wide, tall := planet.Clone(w.project.State), *width, *height
	defer func() {
		io.SetMouseButtonDown(0, false)
		io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
		*width, *height = wide, tall
		w.project.State = saved
		w.bind(w.project)
		w.narrowEditor = false
	}()
	w.project.State.Definition.Settings.Mountain = 1
	w.project.State.Size = 64
	choices := w.project.BiomeChoices(false)
	var base, brush planet.Biome
	for _, b := range choices {
		if b.Path == "/datum/biome/grass" {
			base = b
		}
		if b.Path == "/datum/biome/beach" {
			brush = b
		}
	}
	if base.Path == "" || brush.Path == "" {
		t.Fatal("missing climate fixtures")
	}
	w.project.State.Biomes[base.Path], w.project.State.Biomes[brush.Path] = base, brush
	for row := range w.project.State.Definition.Surface {
		for col := range w.project.State.Definition.Surface[row] {
			w.project.State.Definition.Surface[row][col] = base.Path
		}
	}
	// Starting on an already-painted cell must still allow painting the rest.
	w.project.State.Definition.Surface[2][0] = brush.Path
	w.bind(w.project)
	w.mode = 2
	w.climateView.brush, w.climateView.painting = brush.Path, true
	w.lastBuild = time.Time{}
	capture("climate-painting")
	before, camera := planet.Clone(w.project.State), *w.canvas.Render().Camera
	oldCells := append([]planet.Cell(nil), w.preview.Cells...)
	gridPoint := func(row, col int) imgui.Vec2 {
		return imgui.Vec2{X: w.climateView.gridMin.X + (float32(col)+.5)*w.climateView.cellSize.X, Y: w.climateView.gridMin.Y + (float32(row)+.5)*w.climateView.cellSize.Y}
	}
	io.SetMousePosition(gridPoint(2, 0))
	render()
	if !w.climateView.hoverValid || w.climateView.hover != (planet.ClimateCell{Row: 2, Col: 0}) {
		t.Fatal("grid hover did not identify its rule")
	}
	if !reflect.DeepEqual(before, w.project.State) {
		t.Fatal("hover edited the climate")
	}
	capture("climate-hover")
	io.SetMouseButtonDown(0, true)
	render()
	io.SetMousePosition(gridPoint(2, 4))
	render()
	if !w.climateView.stroke {
		t.Fatal("drag ended before mouse release")
	}
	io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
	io.SetMouseButtonDown(0, false)
	capture("climate-painted")
	for _, path := range w.project.State.Definition.Surface[2] {
		if path != brush.Path {
			t.Fatal("fast drag skipped climate cells")
		}
	}
	if w.climateView.stroke || *w.canvas.Render().Camera != camera || w.project.State.Seeds != before.Seeds {
		t.Fatal("stroke did not finish cleanly or moved the landscape")
	}
	for i, cell := range oldCells {
		after := w.preview.Cells[i]
		if cell.Heat != after.Heat || cell.Moisture != after.Moisture || cell.Height != after.Height || cell.Climate != after.Climate {
			t.Fatal("painting moved the climate fields")
		}
		if cell.Climate.Row != 2 && cell.Biome != after.Biome {
			t.Fatal("painting changed an unrelated rule")
		}
	}
	count, changed := 0, 0
	for cell, n := range w.climateView.counts {
		count += n
		if cell.Row == 2 && cell.Col != 0 && !cell.Caves {
			changed += n
		}
	}
	if count != len(w.preview.Cells) || w.climateCountChanges() != changed {
		t.Fatal("coverage did not match the tiles affected by the stroke")
	}
	w.climateView.lens = 3
	capture("climate-changes")
	w.app.CommandStorage().Undo()
	if !reflect.DeepEqual(before, w.project.State) {
		t.Fatal("one undo did not restore the entire climate stroke")
	}
	w.app.CommandStorage().Redo()
	for _, path := range w.project.State.Definition.Surface[2] {
		if path != brush.Path {
			t.Fatal("redo lost part of a stroke")
		}
	}
	w.app.CommandStorage().Undo()
	w.climateView.painting, w.climateView.lens = false, 0
	capture("climate-inspect")
	index := 25*w.preview.Map.MaxX + 25
	cell := w.preview.Cells[index]
	cam := w.canvas.Render().Camera
	point := imgui.Vec2{X: w.control.PosMin().X + (25.5*32+cam.ShiftX)*cam.Scale, Y: w.control.PosMax().Y - (25.5*32+cam.ShiftY)*cam.Scale}
	io.SetMousePosition(point)
	render()
	io.SetMouseButtonDown(0, true)
	render()
	io.SetMouseButtonDown(0, false)
	render()
	if w.row != cell.Climate.Row || w.col != cell.Climate.Col || w.selected != cell.Biome {
		t.Fatal("map click did not select its climate rule")
	}
	if !reflect.DeepEqual(before, w.project.State) {
		t.Fatal("map inspection edited the planet")
	}
	capture("climate-map-selection")
	// A fast map drag must visit the actual terrain between mouse samples,
	// rather than interpolate through unrelated climate-grid coordinates.
	w.climateView.painting, w.climateView.brush = true, brush.Path
	capture("climate-viewer-brush")
	mapPoint := func(x, y int) imgui.Vec2 {
		cam := w.canvas.Render().Camera
		return imgui.Vec2{X: w.control.PosMin().X + ((float32(x)+.5)*32+cam.ShiftX)*cam.Scale, Y: w.control.PosMax().Y - ((float32(y)+.5)*32+cam.ShiftY)*cam.Scale}
	}
	mapBefore, mapCamera := planet.Clone(w.project.State), *w.canvas.Render().Camera
	var touched map[planet.ClimateCell]bool
	mapRow := 0
	for y := 3; y < w.preview.Map.MaxY-3; y++ {
		row := map[planet.ClimateCell]bool{}
		for x := 3; x < w.preview.Map.MaxX-3; x++ {
			row[w.preview.Cells[y*w.preview.Map.MaxX+x].Climate] = true
		}
		if len(row) > len(touched) {
			touched, mapRow = row, y
		}
	}
	if len(touched) < 3 {
		t.Fatal("map brush fixture does not cross enough climate boundaries")
	}
	io.SetMousePosition(mapPoint(3, mapRow))
	render()
	io.SetMouseButtonDown(0, true)
	render()
	io.SetMousePosition(mapPoint(w.preview.Map.MaxX-4, mapRow))
	render()
	if !w.climateView.stroke || !w.climateView.strokeMap {
		t.Fatal("map brush did not keep its drag active")
	}
	io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
	io.SetMouseButtonDown(0, false)
	capture("climate-viewer-painted")
	for row, cells := range mapBefore.Definition.Surface {
		for col, old := range cells {
			want := old
			if touched[planet.ClimateCell{Row: row, Col: col}] {
				want = brush.Path
			}
			if w.project.State.Definition.Surface[row][col] != want {
				t.Fatalf("map stroke changed the wrong rules at %d,%d", row, col)
			}
		}
	}
	if w.climateView.stroke || *w.canvas.Render().Camera != mapCamera || w.project.State.Seeds != mapBefore.Seeds {
		t.Fatal("map stroke moved the camera or seed, or did not end")
	}
	mapAfter := planet.Clone(w.project.State)
	w.app.CommandStorage().Undo()
	if !reflect.DeepEqual(mapBefore, w.project.State) {
		t.Fatal("one undo did not restore the entire map stroke")
	}
	w.app.CommandStorage().Redo()
	if !reflect.DeepEqual(mapAfter, w.project.State) {
		t.Fatal("redo did not restore the map stroke")
	}
	w.app.CommandStorage().Undo()
	w.lastBuild = time.Time{}
	render()
	// Dragging into the viewer from another control must not start painting.
	io.SetMouseButtonDown(0, true)
	render()
	io.SetMousePosition(mapPoint(25, 25))
	render()
	io.SetMouseButtonDown(0, false)
	render()
	if !reflect.DeepEqual(mapBefore, w.project.State) {
		t.Fatal("entering the viewer with the button held painted terrain")
	}
	// Leaving the viewer breaks the interpolation path while retaining one undo.
	ends := map[planet.ClimateCell]bool{
		w.preview.Cells[mapRow*w.preview.Map.MaxX+3].Climate:                    true,
		w.preview.Cells[mapRow*w.preview.Map.MaxX+w.preview.Map.MaxX-4].Climate: true,
	}
	io.SetMousePosition(mapPoint(3, mapRow))
	render()
	io.SetMouseButtonDown(0, true)
	render()
	io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
	render()
	io.SetMousePosition(mapPoint(w.preview.Map.MaxX-4, mapRow))
	render()
	io.SetMouseButtonDown(0, false)
	render()
	for row, cells := range mapBefore.Definition.Surface {
		for col, old := range cells {
			want := old
			if ends[planet.ClimateCell{Row: row, Col: col}] {
				want = brush.Path
			}
			if w.project.State.Definition.Surface[row][col] != want {
				t.Fatal("map reentry painted across the skipped terrain")
			}
		}
	}
	w.app.CommandStorage().Undo()
	if !reflect.DeepEqual(mapBefore, w.project.State) {
		t.Fatal("reentry split the map stroke's undo")
	}
	io.SetMousePosition(mapPoint(25, 25))
	render()
	io.SetMouseButtonDown(1, true)
	render()
	io.SetMouseButtonDown(1, false)
	render()
	if w.climateView.brush != w.climatePath(w.preview.Cells[index].Climate) || !w.climateView.painting {
		t.Fatal("viewer eyedropper did not select the hovered biome")
	}
	w.climateView.painting, w.climateView.brush = false, brush.Path
	io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
	w.climateView.focused = false
	w.climateView.lens = 1
	capture("climate-heat")
	var previous, minFilter, magFilter int32
	gl.GetIntegerv(gl.TEXTURE_BINDING_2D, &previous)
	gl.BindTexture(gl.TEXTURE_2D, w.climateTexture)
	gl.GetTexParameteriv(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, &minFilter)
	gl.GetTexParameteriv(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, &magFilter)
	gl.BindTexture(gl.TEXTURE_2D, uint32(previous))
	if minFilter != gl.NEAREST || magFilter != gl.NEAREST {
		t.Fatal("climate texture blurs tile edges")
	}
	w.climateView.lens = 2
	capture("climate-moisture")
	w.climateView.lens = 0
	*width, *height = 1000, 760
	w.fit = true
	capture("climate-compact")
	if w.canvasSize.X < 180 || w.canvasSize.Y < 180 {
		t.Fatal("compact climate layout hid the map")
	}
	*width, *height = 800, 760
	w.fit = true
	capture("climate-stacked")
	if w.canvasSize.Y < 100 {
		t.Fatal("stacked climate layout hid the map")
	}
	*width, *height = wide, tall
	w.cave, w.climateView.layer = true, true
	w.row, w.col = 3, 4
	w.fit = true
	capture("climate-caves")
	for _, cell := range w.preview.Cells {
		if !cell.Climate.Caves {
			t.Fatal("cave preview has surface provenance")
		}
	}
	old := planet.Clone(w.project.State)
	w.paintClimate(planet.ClimateCell{Row: 0, Col: 0, Caves: true})
	if !reflect.DeepEqual(old, w.project.State) {
		t.Fatal("surface brush painted cave climate")
	}
	// A mountain's cave rule can be inspected without changing the map layer.
	w.cave, w.climateView.layer = false, false
	w.project.State.Definition.Settings.Mountain = 0
	w.climateView.focused = false
	capture("climate-mountains")
	cam = w.canvas.Render().Camera
	point = imgui.Vec2{X: w.control.PosMin().X + (25.5*32+cam.ShiftX)*cam.Scale, Y: w.control.PosMax().Y - (25.5*32+cam.ShiftY)*cam.Scale}
	io.SetMousePosition(point)
	render()
	io.SetMouseButtonDown(0, true)
	render()
	io.SetMouseButtonDown(0, false)
	render()
	if !w.climateView.layer || w.cave {
		t.Fatal("mountain inspection changed the preview layer")
	}
	capture("climate-mountain-rule")
	old = planet.Clone(w.project.State)
	w.climateView.painting, w.climateView.brush = true, brush.Path
	io.SetMouseButtonDown(0, true)
	render()
	io.SetMouseButtonDown(0, false)
	render()
	if !reflect.DeepEqual(old, w.project.State) {
		t.Fatal("map brush put a surface biome in mountain cave climate")
	}
	w.climateView.painting = false
	io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
	w.project.State.Size = 256
	w.climateView.focused, w.climateView.lens, w.fit = false, 1, true
	capture("climate-large-heat")
	if w.climateView.overlayKey.preview != w.preview {
		t.Fatal("large climate overlay did not update")
	}
}
