package wsmap

import (
	"sdmm/internal/app/prefs"
	"sdmm/internal/dmapi/dmmsave"

	"github.com/rs/zerolog/log"
)

func (ws *WsMap) Save() bool {
	log.Print("saving map workspace:", ws.CommandStackId())

	editorPrefs := ws.app.Prefs().Editor

	var saveFormat dmmsave.Format
	switch editorPrefs.SaveFormat {
	case prefs.SaveFormatInitial:
		saveFormat = dmmsave.FormatInitial
	case prefs.SaveFormatTGM:
		saveFormat = dmmsave.FormatTGM
	case prefs.SaveFormatDMM:
		saveFormat = dmmsave.FormatDM
	}

	if !dmmsave.Save(ws.app.LoadedEnvironment(), ws.paneMap.Dmm(), dmmsave.Config{
		Format:            saveFormat,
		SanitizeVariables: editorPrefs.SanitizeVariables,
	}) {
		return false
	}

	ws.app.CommandStorage().ForceBalance(ws.CommandStackId())
	if app, ok := ws.app.(interface{ MapFileSaved(string) }); ok {
		app.MapFileSaved(ws.paneMap.Dmm().Path.Absolute)
	}
	return true
}
