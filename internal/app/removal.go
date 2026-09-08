package app

import "sdmm/internal/ship"

func (a *app) OnTemplatesRemoved(changes []ship.FileChange) {
	a.layout.Environment.Free()
	a.layout.Prefabs.Free()
	a.layout.VarEditor.Free()
	a.layout.WsArea.TemplatesRemoved(changes)
}
