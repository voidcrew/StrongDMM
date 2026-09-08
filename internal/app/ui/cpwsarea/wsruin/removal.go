package wsruin

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SpaiR/imgui-go"
	"sdmm/internal/app/ui/workshop"
	"sdmm/internal/ship"
)

func (ws *WsRuin) requestRemoval() *workshop.RemovalConfirmation {
	if ws.selected == nil {
		return nil
	}
	catalog, entry := *ws.catalog, *ws.selected
	dme := catalog.Dme
	catalog.Dme = ship.SnapshotRemovalEnvironment(dme)
	r := &workshop.RemovalConfirmation{Kind: "ruin", Entry: entry.Name, Warning: "Unsaved property changes will be discarded. All variants of this entry are included."}
	r.Prepare(func() (*ship.RemovalPlan, error) { return catalog.Removal(entry) })
	r.Check = func() error {
		if ws.app.LoadedEnvironment() != dme {
			return fmt.Errorf("the loaded project changed; review removal again")
		}
		for _, change := range r.Plan.Changes {
			if strings.EqualFold(filepath.Ext(change.Path), ".dmm") && ws.SourceBusy != nil && ws.SourceBusy(change.Path) {
				return fmt.Errorf("close the open map tab for %s before removing this ruin", filepath.Base(change.Path))
			}
		}
		return nil
	}
	r.Removed = ws.finishRemoval
	workshop.OpenRemoval(r)
	return r
}

func (ws *WsRuin) finishRemoval(plan *ship.RemovalPlan, backup string) {
	ws.app.LoadedEnvironment().RemoveTypeTrees(plan.Types)
	ws.project, ws.selected = nil, nil
	ws.form, ws.initial = form{}, form{}
	ws.creating = false
	ws.step = 0
	if app, ok := ws.app.(interface{ OnTemplatesRemoved([]ship.FileChange) }); ok {
		app.OnTemplatesRemoved(plan.Changes)
	} else {
		ws.app.SyncPrefabs()
	}
	ws.refresh()
	ws.message = "Removed " + plan.Name + "."
	ws.removalBackup = backup
}

func (ws *WsRuin) removalRecovery() {
	if ws.removalBackup != "" && imgui.Button("Open recovery folder") {
		workshop.OpenRemovalRecovery(ws.removalBackup)
	}
}

func (ws *WsRuin) RebaseRemoval(changes []ship.FileChange) {
	if ws.project != nil {
		ws.project.RebaseRemoval(changes)
	}
	ws.refresh()
}
