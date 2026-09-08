package wsship

import "github.com/SpaiR/imgui-go"

func (ws *WsShip) beginRename(task buildTask, id, name string) {
	ws.beginTask(task)
	ws.itemID, ws.itemName = id, name
}

func (ws *WsShip) renameScope() string {
	if ws.task == taskRenameTheme {
		return "theme/" + ws.itemID
	}
	return "module/" + ws.itemID
}

func (ws *WsShip) renameControls() {
	if ws.task == taskRenameTheme {
		heading("RENAME SHIP VARIANT")
		textField("Variant name", "e.g. Salvager", &ws.itemName)
	} else {
		heading("RENAME ROOM OPTION")
		textField("Room option name", "e.g. Medical bay", &ws.itemName)
	}
	nameErr := ws.project.RenameNameError(ws.renameScope(), ws.itemName)
	if nameErr != nil {
		hint(nameErr.Error())
	}
	space()
	imgui.BeginDisabledV(nameErr != nil)
	if actionButton("Apply name", true) {
		ws.applyRename()
	}
	imgui.EndDisabled()
	if actionButton("Cancel", false) {
		ws.finishTask()
	}
}

func (ws *WsShip) applyRename() {
	label := "Rename room option"
	if ws.task == taskRenameTheme {
		label = "Rename ship variant"
	}
	ws.change(label, func() error { return ws.project.Rename(ws.renameScope(), ws.itemName) })
	if ws.message == "" {
		ws.finishTask()
	}
}
