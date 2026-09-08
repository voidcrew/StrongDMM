package cpwsarea

import (
	"sdmm/internal/app/ui/cpwsarea/wsruin"
	"sdmm/internal/app/ui/cpwsarea/wsship"
	"sdmm/internal/ship"
)

func (w *WsArea) TemplatesRemoved(changes []ship.FileChange) {
	for _, ws := range w.workspaces {
		switch content := ws.Content().(type) {
		case *wsship.WsShip:
			content.RebaseRemoval(changes)
		case *wsruin.WsRuin:
			content.RebaseRemoval(changes)
		}
	}
}
