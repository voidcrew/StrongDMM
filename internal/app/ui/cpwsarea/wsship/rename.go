package wsship

import (
	"strings"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/window"
)

func (ws *WsShip) beginRename(task buildTask, id, name string) {
	ws.beginTask(task)
	ws.itemID, ws.itemName = id, name
	ws.renameOriginal = name
	ws.itemDescription = ws.project.Description(ws.renameScope())
	ws.descriptionOriginal = ws.itemDescription
}

func (ws *WsShip) renameScope() string {
	if ws.task == taskRenameTheme {
		return "theme/" + ws.itemID
	}
	return "module/" + ws.itemID
}

func (ws *WsShip) renameControls() {
	if ws.task == taskRenameTheme {
		heading("SHIP VARIANT DETAILS")
		textField("Variant name", "e.g. Salvager", &ws.itemName)
	} else {
		heading("ROOM OPTION DETAILS")
		textField("Room option name", "e.g. Medical bay", &ws.itemName)
	}
	descriptionField(&ws.itemDescription)
	nameErr := ws.project.RenameNameError(ws.renameScope(), ws.itemName)
	if nameErr != nil {
		hint(nameErr.Error())
	}
	space()
	imgui.BeginDisabledV(nameErr != nil)
	if actionButton("Apply details", true) {
		ws.applyRename()
	}
	imgui.EndDisabled()
	if actionButton("Cancel", false) {
		ws.cancelRename()
	}
}

func (ws *WsShip) applyRename() {
	if ws.commitRename() {
		ws.task = taskPaint
	}
}

func (ws *WsShip) renamePending() bool {
	return (ws.task == taskRenameTheme || ws.task == taskRenameModule) &&
		(strings.TrimSpace(ws.itemName) != ws.renameOriginal || ws.itemDescription != ws.descriptionOriginal)
}

func (ws *WsShip) cancelRename() {
	ws.task, ws.message = taskPaint, ""
}

func (ws *WsShip) commitRename() bool {
	if !ws.renamePending() {
		return true
	}
	scope, name := ws.renameScope(), ws.itemName
	if err := ws.project.RenameNameError(scope, name); err != nil {
		ws.message = err.Error()
		return false
	}
	label := "Edit room details"
	if ws.task == taskRenameTheme {
		label = "Edit variant details"
	}
	ws.message = ""
	ws.change(label, func() error {
		if err := ws.project.Rename(scope, name); err != nil {
			return err
		}
		return ws.project.SetDescription(scope, ws.itemDescription)
	})
	if ws.message != "" {
		return false
	}
	ws.task = taskPaint
	return true
}

func descriptionField(value *string) {
	imgui.Text("Description")
	imgui.InputTextMultilineV("##component-description", value, imgui.Vec2{X: -1, Y: 95 * window.PointSize()}, 0, nil)
	hint("Shown to players when choosing this option in the shipyard.")
}
