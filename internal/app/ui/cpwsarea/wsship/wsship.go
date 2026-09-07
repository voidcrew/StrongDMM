package wsship

import (
	"fmt"
	"github.com/SpaiR/imgui-go"
	"path/filepath"
	"sdmm/internal/app/command"
	"sdmm/internal/app/ui/cpwsarea/workspace"
	"sdmm/internal/app/ui/cpwsarea/wsmap/pmap"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmminstance"
	"sdmm/internal/ship"
	"sdmm/internal/util"
)

type App interface {
	pmap.App
	OnWorkspaceSwitched()
}
type WsShip struct {
	workspace.Content
	isolated, invalid                bool
	app                              App
	catalog                          *ship.Catalog
	hull, theme, source              int
	selected                         map[string]string
	assembly                         *ship.Assembly
	project                          *ship.Project
	projects                         map[string]*ship.Project
	panes                            map[string]*pmap.PaneMap
	pane                             *pmap.PaneMap
	message                          string
	wizard, focused                  bool
	newID, newName, itemID, itemName string
	width, height                    int32
	rect                             [4]int32
	SourceBusy                       func(string) bool
}

func New(app App, busy ...func(string) bool) *WsShip {
	ws := &WsShip{app: app, projects: map[string]*ship.Project{}, panes: map[string]*pmap.PaneMap{}, width: 32, height: 32, rect: [4]int32{4, 4, 12, 12}}
	if len(busy) > 0 {
		ws.SourceBusy = busy[0]
	}
	var err error
	ws.catalog, err = ship.Discover(app.LoadedEnvironment())
	if err != nil {
		ws.message = err.Error()
		return ws
	}
	ws.app.CommandStorage().SetStack(ws.CommandStackId())
	ws.defaults()
	ws.rebuild()
	return ws
}
func (ws *WsShip) Name() string {
	prefix := ""
	dirty := ws.app.CommandStorage().IsModified(ws.CommandStackId())
	for _, project := range ws.projects {
		for _, d := range project.Documents {
			if d.Active && !d.Existed {
				dirty = true
			}
		}
	}
	if dirty {
		prefix = "* "
	}
	return prefix + "Ship Workshop###" + ws.Id()
}
func (ws *WsShip) Title() string {
	if ws.project != nil {
		return "Ship Workshop - " + ws.project.Hull.Name
	}
	return "Ship Workshop"
}
func (ws *WsShip) Map() *pmap.PaneMap {
	if ws.wizard || ws.invalid {
		return nil
	}
	return ws.pane
}
func (ws *WsShip) CommandStackId() string { return "ship:" + ws.Id() }
func (ws *WsShip) IsModified() bool {
	for _, p := range ws.projects {
		if p.Modified() {
			return true
		}
	}
	return false
}
func (ws *WsShip) PreProcess() {
	for _, p := range ws.panes {
		p.SetShortcutsVisible(false)
	}
}
func (ws *WsShip) Focused() bool {
	return ws.Content.Focused() || (ws.pane != nil && ws.pane.Focused())
}
func (ws *WsShip) OnFocusChange(f bool) {
	ws.focused = f
	if ws.pane != nil && !ws.wizard {
		if f && !ws.invalid {
			ws.pane.OnActivate()
		} else {
			ws.pane.OnDeactivate()
		}
	} else if f {
		tools.SetEnabled(false)
	}
}
func (ws *WsShip) Dispose() {
	if ws.pane != nil {
		ws.pane.OnDeactivate()
	}
	for _, p := range ws.panes {
		p.Dispose()
	}
	ws.app.CommandStorage().DisposeStack(ws.CommandStackId())
}
func (ws *WsShip) currentTheme() ship.Theme {
	h := ws.catalog.Hulls[ws.hull]
	if len(h.Themes) == 0 {
		return ship.Theme{}
	}
	return h.Themes[ws.theme]
}
func (ws *WsShip) defaults() {
	ws.selected = map[string]string{}
	h := ws.catalog.Hulls[ws.hull]
	t := ws.currentTheme()
	for _, slot := range h.SlotsFor(t) {
		for _, m := range h.Modules {
			if m.Slot == slot && m.Default && m.Available(t.ID) {
				ws.selected[slot] = m.ID
				break
			}
		}
	}
	ws.source = 0
}

func (ws *WsShip) rebuild() {
	if ws.catalog == nil {
		return
	}
	h := ws.catalog.Hulls[ws.hull]
	p := ws.projects[h.Type]
	if p == nil {
		var err error
		p, err = ship.OpenProject(ws.catalog, ws.app.LoadedEnvironment(), h)
		if err != nil {
			ws.message = err.Error()
			return
		}
		ws.projects[h.Type] = p
	}
	ws.project = p
	p.BeforeOpen = func(file string) error {
		if ws.SourceBusy != nil && ws.SourceBusy(file) {
			return fmt.Errorf("close the ordinary map tab before opening %s in Ship Workshop", filepath.Base(file))
		}
		return nil
	}
	ws.catalog.Hulls[ws.hull] = p.Hull
	a, err := p.Assemble(ws.currentTheme(), ws.selected)
	if err != nil {
		ws.message = err.Error()
		if !ws.isolated || ws.pane == nil {
			ws.invalid = true
			tools.SetEnabled(false)
			return
		}
		file := ws.pane.Dmm().Path.Absolute
		doc := p.Documents[file]
		if doc == nil || !doc.Active || doc.Map != ws.pane.Dmm() {
			ws.invalid = true
			tools.SetEnabled(false)
			return
		}
		a = &ship.Assembly{Sources: []ship.Source{{Name: "Source repair", File: file, Live: doc.Map, Data: ship.RawData(doc.Map)}}, MaxX: doc.Map.MaxX, MaxY: doc.Map.MaxY, MaxZ: 1}
		ws.source = 0
	}
	if ws.SourceBusy != nil {
		for _, s := range a.Sources {
			if ws.SourceBusy(s.File) {
				ws.message = "Close the ordinary map tab before editing this source in Ship Workshop: " + filepath.Base(s.File)
				return
			}
		}
	}
	var display *dmmap.Dmm
	if !ws.isolated {
		display, err = a.Display(ws.app.LoadedEnvironment())
		if err != nil {
			ws.message = err.Error()
			ws.invalid = true
			tools.SetEnabled(false)
			return
		}
	}
	ws.invalid = false
	if ws.focused {
		tools.SetEnabled(true)
	}
	ws.assembly = a
	if ws.source >= len(a.Sources) {
		ws.source = 0
	}
	for i, s := range a.Sources {
		pane := ws.panes[s.File]
		if pane == nil {
			pane = pmap.New(ws.app, s.Live)
			ws.panes[s.File] = pane
		}
		editable := map[uint64]*dmminstance.Instance{}
		for _, tile := range s.Live.Tiles {
			for _, inst := range tile.Instances() {
				editable[inst.Id()] = inst
			}
		}
		index, theme, hull := i, ws.theme, ws.hull
		selection := copySelection(ws.selected)
		view, offset := display, s.Offset
		if ws.isolated {
			view = s.Live
			offset = util.Point{}
		}
		pane.SetEditContext(&pmap.EditContext{View: view, Offset: offset, StackID: ws.CommandStackId(), Editable: editable, Refresh: ws.refresh, BeforeHistory: func() {
			ws.hull = hull
			ws.theme = theme
			ws.selected = copySelection(selection)
			ws.source = index
			ws.rebuild()
		}, Filter: ws.visible})
	}
	ws.activate(ws.panes[a.Sources[ws.source].File])
	ws.pane.RenderContext()
	ws.message = ""
}
func (ws *WsShip) refresh() { ws.rebuild() }
func copySelection(s map[string]string) map[string]string {
	r := map[string]string{}
	for k, v := range s {
		r[k] = v
	}
	return r
}
func (ws *WsShip) activate(p *pmap.PaneMap) {
	if ws.pane == p {
		return
	}
	if ws.pane != nil {
		p.Canvas().Render().Camera = ws.pane.Canvas().Render().Camera
		ws.pane.OnDeactivate()
	}
	ws.pane = p
	if ws.focused {
		p.OnActivate()
	}
	ws.app.OnWorkspaceSwitched()
}
func (ws *WsShip) visible(path string) bool {
	// Template placeholders are transparent; normal View controls govern all
	// map content, including areas, pipes and cables.
	return path != "/turf/template_noop" && path != "/area/template_noop"
}
func (ws *WsShip) Owns(file string) bool {
	for _, p := range ws.projects {
		if d := p.Documents[file]; d != nil && d.Active {
			return true
		}
	}
	return false
}
func (ws *WsShip) FocusSource(file string) {
	for h, hull := range ws.catalog.Hulls {
		p := ws.projects[hull.Type]
		if p == nil {
			continue
		}
		if d := p.Documents[file]; d == nil || !d.Active {
			continue
		}
		ws.hull = h
		for ti := 0; ti < max(1, len(hull.Themes)); ti++ {
			ws.theme = ti
			ws.defaults()
			ws.rebuild()
			if ws.assembly != nil {
				for i, s := range ws.assembly.Sources {
					if s.File == file {
						ws.source = i
						ws.rebuild()
						return
					}
				}
			}
		}
	}
}
func (ws *WsShip) Save() bool {
	ws.flush()
	projects := []*ship.Project{}
	for _, p := range ws.projects {
		projects = append(projects, p)
	}
	if err := ship.SaveProjects(projects); err != nil {
		ws.message = err.Error()
		return false
	}
	ws.app.CommandStorage().ForceBalance(ws.CommandStackId())
	ws.message = "Ship sources saved."
	return true
}
func (ws *WsShip) flush() {
	tools.FinishStroke()
	for _, p := range ws.panes {
		p.Editor().CommitContextNow("Edit ship source")
	}
}
func (ws *WsShip) change(label string, action func() error) {
	ws.flush()
	p := ws.project
	before := p.Capture()
	h, t := ws.hull, ws.theme
	sel := copySelection(ws.selected)
	if err := action(); err != nil {
		p.Restore(before)
		ws.message = err.Error()
		return
	}
	after := p.Capture()
	restore := func(state ship.State) {
		p.Restore(state)
		ws.hull = h
		ws.theme = t
		ws.selected = copySelection(sel)
		ws.source = 0
		ws.catalog.Hulls[h] = p.Hull
		for _, pane := range ws.panes {
			pane.Snapshot().Sync()
			pane.CanvasState().SetMaxX(pane.Dmm().MaxX)
			pane.CanvasState().SetMaxY(pane.Dmm().MaxY)
		}
		ws.rebuild()
	}
	for _, pane := range ws.panes {
		pane.Snapshot().Sync()
		pane.CanvasState().SetMaxX(pane.Dmm().MaxX)
		pane.CanvasState().SetMaxY(pane.Dmm().MaxY)
	}
	ws.app.CommandStorage().PushV(ws.CommandStackId(), command.Make(label, func() { restore(before) }, func() { restore(after) }))
	ws.catalog.Hulls[h] = p.Hull
	ws.rebuild()
}
func (ws *WsShip) Process() {
	imgui.BeginChildV("ship-controls", imgui.Vec2{X: 320}, true, 0)
	ws.controls()
	imgui.EndChild()
	imgui.SameLine()
	imgui.BeginChildV("ship-canvas", imgui.Vec2{}, false, imgui.WindowFlagsNoScrollbar|imgui.WindowFlagsNoScrollWithMouse)
	if ws.wizard {
		ws.newShip()
	} else if ws.pane != nil && !ws.invalid {
		ws.pane.Process()
	} else {
		imgui.TextWrapped(ws.message)
	}
	imgui.EndChild()
}
func (ws *WsShip) controls() {
	imgui.Text("SHIP WORKSHOP")
	if imgui.Button("New ship...") {
		ws.flush()
		ws.OnFocusChange(false)
		ws.wizard = true
		tools.SetEnabled(false)
	}
	imgui.SameLine()
	if imgui.Button("Save all ships") {
		ws.Save()
	}
	if ws.catalog == nil {
		imgui.TextWrapped(ws.message)
		return
	}
	imgui.PushItemWidth(-1)
	if imgui.BeginCombo("##ship", ws.catalog.Hulls[ws.hull].Name) {
		for i, h := range ws.catalog.Hulls {
			if imgui.SelectableV(h.Name, i == ws.hull, 0, imgui.Vec2{}) {
				ws.flush()
				ws.wizard = false
				ws.hull = i
				ws.theme = 0
				ws.defaults()
				ws.rebuild()
				ws.OnFocusChange(true)
			}
		}
		imgui.EndCombo()
	}
	if ws.project == nil {
		imgui.PopItemWidth()
		return
	}
	h := ws.project.Hull
	if len(h.Themes) > 0 && imgui.BeginCombo("Theme", ws.currentTheme().Name) {
		for i, t := range h.Themes {
			if imgui.SelectableV(t.Name, i == ws.theme, 0, imgui.Vec2{}) {
				ws.flush()
				ws.theme = i
				ws.defaults()
				ws.rebuild()
			}
		}
		imgui.EndCombo()
	}
	for _, slot := range h.SlotsFor(ws.currentTheme()) {
		label := "Bare slot"
		for _, m := range h.Modules {
			if m.ID == ws.selected[slot] {
				label = m.Name
			}
		}
		if imgui.BeginCombo(slot, label) {
			if imgui.Selectable("Bare slot") {
				ws.flush()
				ws.selected[slot] = ""
				ws.source = 0
				ws.rebuild()
			}
			for _, m := range h.Modules {
				if m.Slot == slot && m.Available(ws.currentTheme().ID) && imgui.SelectableV(m.Name, ws.selected[slot] == m.ID, 0, imgui.Vec2{}) {
					ws.flush()
					ws.selected[slot] = m.ID
					ws.source = 0
					ws.rebuild()
				}
			}
			imgui.EndCombo()
		}
	}
	imgui.Separator()
	imgui.Text("Editing")
	if imgui.Checkbox("Source alone", &ws.isolated) {
		ws.rebuild()
		if ws.pane != nil {
			ws.pane.FitView()
		}
	}
	if ws.assembly != nil {
		for i, s := range ws.assembly.Sources {
			if imgui.SelectableV(s.Name, i == ws.source, 0, imgui.Vec2{}) {
				ws.flush()
				ws.source = i
				ws.rebuild()
			}
		}
		if ws.source < len(ws.assembly.Sources) {
			s := ws.assembly.Sources[ws.source]
			rel, _ := filepath.Rel(ws.catalog.Root, s.File)
			imgui.TextWrapped(filepath.ToSlash(rel))
			imgui.TextDisabled(fmt.Sprintf("Source %d x %d; offset %d, %d", s.Data.MaxX, s.Data.MaxY, s.Offset.X, s.Offset.Y))
		}
	}
	if imgui.Button("Fit view") && ws.pane != nil {
		ws.pane.FitView()
	}
	if ws.project.Settings != nil {
		ws.authorControls()
	}
	if imgui.CollapsingHeader("Save preview") {
		changes, err := ws.project.Changes()
		if err != nil {
			imgui.TextWrapped(err.Error())
		} else {
			for _, c := range changes {
				rel, _ := filepath.Rel(ws.catalog.Root, c.Path)
				prefix := "Edit "
				if !c.Existed {
					prefix = "New "
				}
				imgui.TextWrapped(prefix + filepath.ToSlash(rel))
			}
			if len(changes) == 0 {
				imgui.TextDisabled("No changes")
			}
		}
	}
	if imgui.CollapsingHeader("Checks") {
		if imgui.Button("Check current loadout") {
			ws.rebuild()
		}
		if ws.assembly != nil {
			for _, issue := range ws.assembly.Issues {
				imgui.TextWrapped(issue.Message)
			}
		}
		imgui.TextWrapped("Save drafts at any stage. Compile, regenerate purchase previews and playtest before shipping.")
	}
	if ws.message != "" {
		imgui.Separator()
		imgui.TextWrapped(ws.message)
	}
	imgui.PopItemWidth()
}
