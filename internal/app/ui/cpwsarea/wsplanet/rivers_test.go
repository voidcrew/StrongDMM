package wsplanet

import (
	"strings"
	"testing"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/planet"
)

func testRiverPreview(t *testing.T, w *Workspace, io imgui.IO, render func(), capture func(string)) {
	t.Helper()
	original := w.project
	defer func() {
		io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
		w.switchPlanet(original.State.Definition.Path)
	}()
	for _, test := range []struct{ planet, name, turf string }{
		{"/datum/planet/lava", "lava-rivers", "/turf/open/lava/smooth/lava_land_surface/planetary"},
		{"/datum/planet/snow", "plasma-rivers", "/turf/open/lava/plasma/planetary"},
	} {
		w.switchPlanet(test.planet)
		w.mode, w.cave = 0, false
		w.fit = true
		capture(test.name)
		if w.preview == nil || planet.RiverTurf(w.catalog, w.project.State) != test.turf {
			t.Fatal("missing environment river configuration", test.name, w.message)
		}
		seen := map[string]bool{}
		for _, tile := range w.scene.Map.Tiles {
			for _, instance := range tile.Instances() {
				p := instance.Prefab()
				if p.Path() != test.turf && !strings.HasPrefix(p.Path(), "/turf/closed/mineral") {
					continue
				}
				v := p.Vars()
				icon, state := v.TextV("icon", ""), v.TextV("icon_state", "")
				key := icon + " / " + state
				if seen[key] {
					continue
				}
				seen[key] = true
				d, err := dmicon.Cache.Get(icon)
				if err != nil || d.States[state] == nil {
					t.Error("missing terrain appearance", p.Path(), key, err)
				}
			}
		}
		found := false
		camera := w.canvas.Render().Camera
		for i, cell := range w.preview.Cells {
			if !cell.River {
				continue
			}
			x, y := i%w.preview.Map.MaxX, i/w.preview.Map.MaxX
			if x < 6 || y < 6 || x >= w.preview.Map.MaxX-6 || y >= w.preview.Map.MaxY-6 {
				continue
			}
			pos := imgui.Vec2{X: w.control.PosMin().X + (float32(x*32+16)+camera.ShiftX)*camera.Scale, Y: w.control.PosMax().Y - (float32(y*32+16)+camera.ShiftY)*camera.Scale}
			io.SetMousePosition(pos)
			render()
			if target, ok := w.previewTarget(); !ok || target.field != "river" {
				continue
			}
			io.SetMouseButtonDown(0, true)
			render()
			io.SetMouseButtonDown(0, false)
			render()
			found = true
			break
		}
		if !found || w.mode != 3 || !w.narrowEditor {
			t.Fatal("river click did not open terrain settings", test.name)
		}
		io.SetMousePosition(imgui.Vec2{X: -1000, Y: -1000})
		capture(test.name + "-settings")
		w.mode, w.cave = 0, true
		capture(test.name + "-underground")
		for _, cell := range w.preview.Cells {
			if cell.River {
				t.Fatal("surface rivers appeared underground")
			}
		}
	}
}
