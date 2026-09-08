package cpwsarea

import (
	"testing"

	"sdmm/internal/app/command"
	"sdmm/internal/app/ui/cpwsarea/workspace"
)

type closeTestApp struct {
	App
	commands *command.Storage
}

func (a *closeTestApp) CommandStorage() *command.Storage { return a.commands }

type closeTestContent struct {
	workspace.Content
	saveOK   bool
	saves    int
	disposed bool
}

func (*closeTestContent) Name() string  { return "unsaved map" }
func (*closeTestContent) Title() string { return "unsaved map" }
func (c *closeTestContent) Save() bool  { c.saves++; return c.saveOK }
func (c *closeTestContent) Dispose()    { c.disposed = true }

// Updates use the same close confirmation as Exit. Cancellation and a failed
// save must keep all workspaces alive and deny permission to restart.
func TestCloseConfirmationProtectsUnsavedWork(t *testing.T) {
	for _, choice := range []string{"cancel", "failed save", "save", "discard"} {
		t.Run(choice, func(t *testing.T) {
			first := &closeTestContent{saveOK: true}
			second := &closeTestContent{saveOK: choice != "failed save"}
			workspaces := []*workspace.Workspace{workspace.New(first), workspace.New(second)}
			area := &WsArea{app: &closeTestApp{commands: command.NewStorage()}, workspaces: workspaces}
			called, accepted := 0, false
			confirmation := area.makeCloseWorkspacesDialog(workspaces, workspaces, func(ok bool) { called++; accepted = ok })
			switch choice {
			case "cancel":
				confirmation.ActionCancel()
			case "discard":
				confirmation.ActionNo()
			default:
				confirmation.ActionYes()
			}
			wantClose := choice == "save" || choice == "discard"
			if called != 1 || accepted != wantClose || first.disposed != wantClose || second.disposed != wantClose {
				t.Fatalf("unsafe close: accepted=%v, disposed=%v/%v, callback=%d", accepted, first.disposed, second.disposed, called)
			}
			if !wantClose && len(area.workspaces) != 2 {
				t.Fatal("unsaved workspaces were removed")
			}
			if (choice == "cancel" || choice == "discard") && (first.saves != 0 || second.saves != 0) {
				t.Fatal("saved without choosing Save")
			}
		})
	}
}
