package app

import (
	"sdmm/internal/app/window"
	"sdmm/internal/dmapi/dmmap"
)

func (a *app) DoPreviewMap(source *dmmap.Dmm) {
	// Open the tab after the current workspace iteration has finished.
	window.RunLater(func() { a.layout.WsArea.OpenPreview(source) })
}
