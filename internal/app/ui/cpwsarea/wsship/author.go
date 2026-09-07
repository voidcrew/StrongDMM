package wsship

import (
	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/ship"
	"sdmm/internal/util"
)

func (ws *WsShip) newShip() {
	imgui.Text("Create a new ship")
	imgui.TextWrapped("Start with an empty canvas and a docking port. Draw the permanent hull, then select rooms to turn into upgrade slots.")
	imgui.InputText("Ship ID", &ws.newID)
	imgui.InputText("Name", &ws.newName)
	imgui.InputInt("Width", &ws.width)
	imgui.InputInt("Height", &ws.height)
	if imgui.Button("Create ship") {
		p, err := ship.NewProject(ws.catalog, ws.app.LoadedEnvironment(), ws.newID, ws.newName, int(ws.width), int(ws.height))
		if err != nil {
			ws.message = err.Error()
		} else {
			ws.projects[p.Hull.Type] = p
			ws.catalog.Hulls = append(ws.catalog.Hulls, p.Hull)
			ws.hull = len(ws.catalog.Hulls) - 1
			ws.theme = 0
			ws.defaults()
			ws.wizard = false
			ws.rebuild()
			ws.OnFocusChange(true)
			ws.pane.FitView()
			ws.app.DoSelectPrefab(dmmap.PrefabStorage.Initial("/turf/open/floor/plating"))
		}
	}
	imgui.SameLine()
	if imgui.Button("Cancel") {
		ws.wizard = false
		ws.OnFocusChange(true)
	}
	if ws.message != "" {
		imgui.TextWrapped(ws.message)
	}
}
func (ws *WsShip) authorControls() {
	if imgui.CollapsingHeader("Build") {
		imgui.InputText("ID##item", &ws.itemID)
		imgui.InputText("Name##item", &ws.itemName)
		if imgui.Button("Use selection bounds") && ws.source == 0 {
			points := tools.SelectedTiles()
			if len(points) > 0 {
				lo, hi := points[0], points[0]
				for _, p := range points {
					lo.X = min(lo.X, p.X)
					lo.Y = min(lo.Y, p.Y)
					hi.X = max(hi.X, p.X)
					hi.Y = max(hi.Y, p.Y)
				}
				ws.rect = [4]int32{int32(lo.X), int32(lo.Y), int32(hi.X), int32(hi.Y)}
			}
		}
		imgui.InputInt("Left", &ws.rect[0])
		imgui.InputInt("Bottom", &ws.rect[1])
		imgui.InputInt("Right", &ws.rect[2])
		imgui.InputInt("Top", &ws.rect[3])
		lo := util.Point{X: int(ws.rect[0]), Y: int(ws.rect[1]), Z: 1}
		hi := util.Point{X: int(ws.rect[2]), Y: int(ws.rect[3]), Z: 1}
		imgui.BeginDisabledV(ws.source != 0)
		if imgui.Button("Lay permanent deck") {
			ws.change("Lay permanent deck", func() error { return ws.project.Deck(ws.currentTheme(), lo, hi) })
		}
		if imgui.Button("Extract upgrade slot") {
			ws.change("Extract upgrade slot", func() error { return ws.project.AddSlot(ws.theme, ws.itemID, ws.itemName, lo, hi) })
			if ws.message == "" {
				ws.defaults()
				ws.rebuild()
			}
		}
		imgui.EndDisabled()
		if imgui.Button("Clone theme") {
			ws.change("Clone theme", func() error { return ws.project.AddTheme(ws.theme, ws.itemID, ws.itemName) })
		}
		if ws.source > 0 && ws.assembly != nil {
			slot := ws.assembly.Sources[ws.source].Slot
			for _, m := range ws.project.Hull.Modules {
				if m.ID == ws.selected[slot] {
					if imgui.Button("Clone module") {
						ws.change("Clone module", func() error { return ws.project.AddModule(ws.theme, m, ws.itemID, ws.itemName, false) })
					}
					if imgui.Button("New empty alternative") {
						ws.change("Create module", func() error { return ws.project.AddModule(ws.theme, m, ws.itemID, ws.itemName, true) })
					}
					break
				}
			}
		}
	}
	if imgui.CollapsingHeader("Ship settings") {
		s := ws.project.Settings
		name, desc := ws.project.Hull.Name, s.Description
		crew, cost, dir := int32(s.Crew), int32(s.Cost), int32(s.PortDirection)
		hidden := s.Hidden
		changed := imgui.InputText("Ship name", &name)
		changed = imgui.InputText("Description", &desc) || changed
		changed = imgui.InputInt("Crew", &crew) || changed
		changed = imgui.InputInt("Misc part cost", &cost) || changed
		changed = imgui.InputInt("Dock: N1 S2 E4 W8", &dir) || changed
		changed = imgui.Checkbox("Development only", &hidden) || changed
		if changed {
			ws.change("Ship settings", func() error {
				ws.project.Hull.Name = name
				s.Description = desc
				s.Crew = int(crew)
				s.Cost = int(cost)
				s.PortDirection = int(dir)
				s.Hidden = hidden
				return nil
			})
		}
		imgui.InputInt("Canvas width", &ws.width)
		imgui.InputInt("Canvas height", &ws.height)
		if imgui.Button("Resize hull") {
			ws.change("Resize hull", func() error { return ws.project.Resize(ws.currentTheme(), int(ws.width), int(ws.height)) })
		}
	}
}
