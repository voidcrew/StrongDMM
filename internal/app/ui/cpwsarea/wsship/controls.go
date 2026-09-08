package wsship

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/cpwsarea/wsmap/tools"
	"sdmm/internal/app/window"
)

const (
	stepChoose = iota
	stepBuild
	stepReview
)

type reviewProject struct {
	name, id, error string
	files           []reviewFile
}
type reviewFile struct {
	path    string
	existed bool
}

func (ws *WsShip) prepareReview() {
	ws.reviewed = nil
	for _, h := range ws.catalog.Hulls {
		p := ws.projects[h.Type]
		if p == nil {
			continue
		}
		changes, err := p.Changes()
		item := reviewProject{name: p.Hull.Name, id: h.Type}
		if err != nil {
			item.error = err.Error()
		}
		for _, c := range changes {
			rel, _ := filepath.Rel(ws.catalog.Root, c.Path)
			item.files = append(item.files, reviewFile{filepath.ToSlash(rel), c.Existed})
		}
		if len(item.files) > 0 || err != nil {
			ws.reviewed = append(ws.reviewed, item)
		}
	}
	ws.reviewReady = true
}

func space() { imgui.Dummy(imgui.Vec2{Y: 6 * window.PointSize()}) }
func heading(text string) {
	space()
	imgui.TextColored(imgui.Vec4{X: .45, Y: .81, Z: .83, W: 1}, text)
}
func title(text string) {
	space()
	imgui.PushFont(window.FontH2)
	imgui.TextWrapped(text)
	imgui.PopFont()
	space()
}
func hint(text string) {
	imgui.PushStyleColor(imgui.StyleColorText, imgui.CurrentStyle().Color(imgui.StyleColorTextDisabled))
	imgui.TextWrapped(text)
	imgui.PopStyleColor()
}
func tooltip(text string) {
	if imgui.IsItemHovered() {
		imgui.BeginTooltip()
		imgui.PushTextWrapPosV(300 * window.PointSize())
		imgui.TextWrapped(text)
		imgui.PopTextWrapPos()
		imgui.EndTooltip()
	}
}
func textField(label, placeholder string, value *string) bool {
	imgui.Text(label)
	imgui.PushItemWidth(-1)
	changed := imgui.InputTextWithHint("##"+label, placeholder, value)
	imgui.PopItemWidth()
	return changed
}
func numberField(label string, value *int32) {
	imgui.Text(label)
	imgui.PushItemWidth(-1)
	imgui.InputInt("##"+label, value)
	imgui.PopItemWidth()
}
func combo(label, preview string) bool {
	return comboHelp(label, preview, "")
}
func comboHelp(label, preview, help string) bool {
	imgui.Text(label)
	if help != "" {
		tooltip(help)
	}
	// An open combo switches to the popup window. Set the next item's width
	// without leaving a width-stack entry to pop from the wrong window.
	imgui.SetNextItemWidth(-1)
	open := imgui.BeginCombo("##"+label, preview)
	if help != "" {
		if open {
			hint(help)
			imgui.Separator()
		} else {
			tooltip(help)
		}
	}
	return open
}
func actionButton(label string, primary bool) bool {
	if primary {
		imgui.PushStyleColor(imgui.StyleColorButton, imgui.Vec4{X: .12, Y: .39, Z: .41, W: 1})
		imgui.PushStyleColor(imgui.StyleColorButtonHovered, imgui.Vec4{X: .16, Y: .49, Z: .51, W: 1})
		imgui.PushStyleColor(imgui.StyleColorButtonActive, imgui.Vec4{X: .09, Y: .32, Z: .34, W: 1})
	}
	clicked := imgui.ButtonV(label, imgui.Vec2{X: -1, Y: 30 * window.PointSize()})
	if primary {
		imgui.PopStyleColorV(3)
	}
	return clicked
}

func (ws *WsShip) setStage(stage int) {
	ws.flush()
	ws.OnFocusChange(false)
	ws.stage, ws.wizard = stage, false
	ws.task = taskPaint
	if tools.IsSelected(tools.TNRegion) {
		tools.SetSelected(tools.TNAdd)
	}
	if stage == stepReview {
		ws.rebuild()
	}
	ws.OnFocusChange(true)
}

func (ws *WsShip) Process() {
	scale := window.PointSize()
	imgui.PushStyleVarVec2(imgui.StyleVarWindowPadding, imgui.Vec2{X: 14 * scale, Y: 12 * scale})
	imgui.PushStyleVarVec2(imgui.StyleVarItemSpacing, imgui.Vec2{X: 8 * scale, Y: 7 * scale})
	imgui.PushStyleVarVec2(imgui.StyleVarFramePadding, imgui.Vec2{X: 8 * scale, Y: 5 * scale})
	imgui.PushStyleVarFloat(imgui.StyleVarFrameRounding, 3*scale)
	imgui.BeginChildV("ship-controls", imgui.Vec2{X: 320 * scale}, true, imgui.WindowFlagsAlwaysUseWindowPadding)
	ws.controls()
	imgui.EndChild()
	imgui.SameLine()
	flags := imgui.WindowFlagsAlwaysUseWindowPadding
	if ws.stage == stepBuild && !ws.wizard {
		flags |= imgui.WindowFlagsNoScrollbar | imgui.WindowFlagsNoScrollWithMouse
	}
	imgui.BeginChildV("ship-content", imgui.Vec2{}, false, flags)
	switch {
	case ws.wizard:
		ws.newShip()
	case ws.stage == stepChoose:
		ws.chooseShip()
	case ws.stage == stepReview:
		ws.review()
	case ws.pane != nil && !ws.invalid:
		ws.canvasHeader()
		ws.pane.Process()
	default:
		imgui.TextWrapped(ws.message)
	}
	imgui.EndChild()
	imgui.PopStyleVarV(4)
}

func (ws *WsShip) controls() {
	heading("SHIP WORKSHOP")
	if ws.project != nil && ws.stage != stepChoose && !ws.wizard {
		imgui.TextWrapped(ws.project.Hull.Name)
	} else {
		hint("Ships, modules, themes and areas.")
	}
	space()
	for i, label := range []string{"1   Choose a ship", "2   Build", "3   Review & save"} {
		imgui.BeginDisabledV(i != stepChoose && (ws.project == nil || ws.wizard))
		if actionButton(label, ws.stage == i) {
			ws.setStage(i)
		}
		imgui.EndDisabled()
	}
	space()
	imgui.Separator()
	if ws.catalog == nil {
		imgui.TextWrapped(ws.message)
		return
	}
	if ws.wizard {
		heading("NEW SHIP")
		hint("Name your ship, then choose its starting canvas.")
		return
	}
	switch ws.stage {
	case stepChoose:
		heading("START HERE")
		hint("Create a ship from scratch, or open an existing ship to work on it.")
	case stepReview:
		heading("READY TO SAVE?")
		hint("Review the checks and changed ships. Saving writes your work to the project.")
		space()
		if actionButton("Back to building", false) {
			ws.setStage(stepBuild)
		}
	default:
		ws.buildControls()
	}
	if ws.message != "" && ws.stage != stepReview {
		space()
		imgui.Separator()
		imgui.TextWrapped(ws.message)
	}
}

func (ws *WsShip) chooseShip() {
	title("What would you like to work on?")
	hint("Open a ship to edit it on the map. You can return here at any time.")
	space()
	if actionButton("Create a new ship...", true) {
		ws.BeginNewShip()
	}
	space()
	heading("EXISTING SHIPS")
	textField("Find a ship", "Search by name", &ws.shipFilter)
	if ws.catalog == nil {
		return
	}
	imgui.BeginChild("ship-list")
	imgui.PushStyleVarVec2(imgui.StyleVarSelectableTextAlign, imgui.Vec2{X: 0, Y: .5})
	for i, h := range ws.catalog.Hulls {
		if !strings.Contains(strings.ToLower(h.Name), strings.ToLower(ws.shipFilter)) {
			continue
		}
		imgui.PushID(h.Type)
		if imgui.SelectableV(h.Name, i == ws.hull, 0, imgui.Vec2{Y: 32 * window.PointSize()}) {
			ws.flush()
			ws.hull, ws.theme = i, 0
			ws.isolated = false
			ws.defaults()
			ws.rebuild()
			ws.setStage(stepBuild)
			if ws.pane != nil {
				ws.pane.FitView()
			}
		}
		imgui.PopID()
	}
	imgui.PopStyleVar()
	imgui.EndChild()
}

func (ws *WsShip) buildControls() {
	if ws.project == nil {
		return
	}
	if ws.task != taskPaint {
		ws.authorControls()
		return
	}
	heading("EDIT SHIP")
	if ws.assembly != nil && ws.source < len(ws.assembly.Sources) && comboHelp("Part to edit", ws.assembly.Sources[ws.source].Name, "Choose where your map edits go: the hull or a displayed room option. The displayed room options stay the same.") {
		for i, s := range ws.assembly.Sources {
			if imgui.SelectableV(s.Name, i == ws.source, 0, imgui.Vec2{}) {
				ws.flush()
				ws.source = i
				ws.rebuild()
			}
			if i == 0 {
				tooltip("Edit the hull, including its floors, walls and permanent equipment.")
			} else {
				tooltip("Edit " + s.Name + ". Changes are saved to this room option.")
			}
		}
		imgui.EndCombo()
	}
	if ws.assembly != nil && ws.source < len(ws.assembly.Sources) {
		if ws.source == 0 {
			hint("Your edits affect the hull: floors, walls and permanent equipment.")
		} else {
			hint("Your edits affect the " + ws.assembly.Sources[ws.source].Name + " room option.")
		}
	}
	space()
	areaAction := "Ship areas..."
	if _, _, selected := tools.SelectionBounds(); selected {
		areaAction = "Make or assign an area..."
	}
	if actionButton(areaAction, false) {
		ws.beginTask(taskArea)
	}
	if actionButton("Set up docking port...", false) {
		ws.beginTask(taskDocking)
	}
	tooltip("Select an entrance with Grab (3), then place or move this ship's mobile docking port there.")
	if actionButton("Make an upgrade room...", false) {
		ws.beginTask(taskRoom)
	}
	space()
	if imgui.CollapsingHeader("Room options & ship variants") {
		ws.loadoutControls()
	}
	if ws.project.Settings != nil && imgui.CollapsingHeader("Ship details & canvas size") {
		if actionButton("Edit ship details...", false) {
			ws.beginTask(taskSettings)
		}
		if actionButton("Change canvas size...", false) {
			ws.beginTask(taskResize)
		}
	}
	if imgui.CollapsingHeader("Advanced view & source files") {
		if imgui.Checkbox("Show only the part being edited", &ws.isolated) {
			ws.flush()
			ws.rebuild()
			if ws.pane != nil {
				ws.pane.FitView()
			}
		}
		if ws.assembly != nil && ws.source < len(ws.assembly.Sources) {
			s := ws.assembly.Sources[ws.source]
			rel, _ := filepath.Rel(ws.catalog.Root, s.File)
			hint(filepath.ToSlash(rel))
			hint(fmt.Sprintf("%d x %d tiles; placed at %d, %d", s.Data.MaxX, s.Data.MaxY, s.Offset.X, s.Offset.Y))
		}
	}
	space()
	if actionButton("Continue to review & save", true) {
		ws.setStage(stepReview)
	}
}

func (ws *WsShip) canvasHeader() {
	if imgui.Button("Show whole ship") {
		if ws.isolated {
			ws.flush()
			ws.isolated = false
			ws.rebuild()
		}
		ws.pane.FitView()
	}
	tooltip("Centers the ship and adjusts the zoom so it fits on screen.")
	imgui.SameLine()
	areas := ws.app.PathsFilter().IsVisiblePath("/area")
	if imgui.Checkbox("Areas", &areas) {
		ws.app.PathsFilter().TogglePath("/area")
	}
	tooltip("Show area markers. This is the same setting as View > Areas (Ctrl+1).")
	imgui.SameLine()
	if tools.IsSelected(tools.TNGrab) && (ws.task == taskArea || ws.task == taskRoom || ws.task == taskDocking) {
		imgui.Text("Select tiles in the part being edited")
	} else if ws.assembly != nil && ws.source < len(ws.assembly.Sources) {
		imgui.Text("Editing: " + ws.assembly.Sources[ws.source].Name)
	}
	if tools.IsSelected(tools.TNGrab) && (ws.task == taskArea || ws.task == taskRoom || ws.task == taskDocking) {
		hint("Grab selection (3) is used by the action in the left panel.")
	} else {
		instruction := "Scroll to zoom  |  Middle mouse to pan  |  Ctrl+Z to undo"
		if p, ok := ws.app.SelectedPrefab(); ok && tools.IsSelected(tools.TNAdd) {
			name, err := strconv.Unquote(p.Vars().ValueV("name", ""))
			if err != nil || name == "" {
				name = filepath.Base(p.Path())
			}
			instruction = "Placing: " + name + "  |  " + instruction
		}
		hint(instruction)
	}
	imgui.Separator()
}

func (ws *WsShip) loadoutControls() {
	h := ws.project.Hull
	if len(h.Themes) > 1 && combo("Ship variant", ws.currentTheme().Name) {
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
		label := "Empty room"
		for _, m := range h.Modules {
			if m.ID == ws.selected[slot] {
				label = m.Name
			}
		}
		if comboHelp("Room: "+slot, label, "Choose which room option is shown here. Choosing it also makes it the part you edit.") {
			if imgui.Selectable("Empty room") {
				ws.selectRoomOption(slot, "")
			}
			tooltip("Remove the room option from this preview and switch editing to the hull.")
			for _, m := range h.Modules {
				if m.Slot != slot || !m.Available(ws.currentTheme().ID) {
					continue
				}
				if imgui.SelectableV(m.Name, ws.selected[slot] == m.ID, 0, imgui.Vec2{}) {
					ws.selectRoomOption(slot, m.ID)
				}
				tooltip("Show and edit " + m.Name + " in this room.")
			}
			imgui.EndCombo()
		}
	}
	if ws.project.Settings != nil && actionButton("Copy ship as a new variant...", false) {
		ws.beginTask(taskTheme)
	}
	if ws.source > 0 {
		if actionButton("Create another room option...", false) {
			ws.beginTask(taskModule)
		}
	}
}

func (ws *WsShip) selectRoomOption(slot, id string) {
	ws.flush()
	ws.selected[slot] = id
	ws.source = 0
	ws.rebuild()
	if id == "" || ws.invalid || ws.assembly == nil {
		return
	}
	for i, source := range ws.assembly.Sources {
		if source.Slot == slot {
			ws.source = i
			ws.rebuild()
			return
		}
	}
}

func (ws *WsShip) review() {
	if ws.project == nil {
		return
	}
	title("Review & save")
	imgui.TextWrapped(ws.project.Hull.Name)
	hint("You can save a draft at any point and keep building later.")
	space()
	if actionButton("Save all changes", true) {
		ws.Save()
	}
	if ws.message != "" {
		imgui.TextWrapped(ws.message)
	}
	heading("MAP CHECKS")
	if ws.invalid || ws.assembly == nil {
		imgui.TextWrapped("The ship could not be assembled. Return to Build to resolve the reported problem.")
	} else if len(ws.assembly.Issues) == 0 {
		imgui.Text("No assembly warnings for this combination.")
	} else {
		imgui.Text(fmt.Sprintf("%d things to check", len(ws.assembly.Issues)))
		for _, issue := range ws.assembly.Issues {
			imgui.BulletText(issue.Message)
		}
	}
	heading("CHANGES TO SAVE")
	if !ws.reviewReady {
		ws.prepareReview()
	}
	for _, item := range ws.reviewed {
		if item.error != "" {
			imgui.TextWrapped(item.name + ": " + item.error)
			continue
		}
		imgui.Text(fmt.Sprintf("%s - %d files", item.name, len(item.files)))
		if imgui.TreeNode("Show files##" + item.id) {
			for _, c := range item.files {
				prefix := "Update "
				if !c.existed {
					prefix = "Create "
				}
				hint(prefix + c.path)
			}
			imgui.TreePop()
		}
	}
	if len(ws.reviewed) == 0 {
		hint("All changes are saved.")
	}
	space()
	hint("Before using a new ship in-game: compile the project, generate purchase previews, and playtest.")
}
