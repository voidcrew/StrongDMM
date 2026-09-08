package wsship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sdmm/internal/app/ui/dialog"
	"sdmm/internal/dmapi/dmenv"
)

func exerciseRemoval(t *testing.T, ws *WsShip, render func()) {
	t.Helper()
	h := ws.project.Hull
	root := ws.catalog.Root
	if !strings.HasSuffix(h.Type, "/workshop_fixture") {
		t.Fatal("removal must only target the generated fixture")
	}
	r := ws.requestRemoval(h)
	if r == nil {
		t.Fatal("removal did not open")
	}
	deadline := time.Now().Add(2 * time.Minute)
	for r.Pending() && time.Now().Before(deadline) {
		render()
		time.Sleep(10 * time.Millisecond)
	}
	if r.Pending() || r.Error != "" || r.Plan == nil {
		t.Fatal("removal preview failed", r.Error)
	}
	for i := 0; i < 3; i++ {
		render()
	}
	if dst := os.Getenv("SHIP_RENDER_TEST_OUTPUT"); dst != "" {
		captureFrame(t, filepath.Join(dst, "remove-ship.png"), 1400, 960)
	}
	for _, change := range r.Plan.Changes {
		rel, err := filepath.Rel(root, change.Path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatal("removal escaped the fixture")
		}
	}
	ws.SourceBusy = func(string) bool { return true }
	if err := r.Confirm(); err == nil {
		t.Fatal("removed a map that is still open")
	}
	ws.SourceBusy = nil
	if err := r.Confirm(); err != nil {
		t.Fatal(err)
	}
	dialog.Close(r)
	if ws.project != nil || ws.Map() != nil || ws.IsModified() {
		t.Fatal("removed ship remains active")
	}
	if _, err := os.Stat(ws.removalBackup); err != nil {
		t.Fatal("recovery missing", err)
	}
	parsed, err := dmenv.New(ws.app.LoadedEnvironment().RootFile)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Objects[h.Type] != nil {
		t.Fatal("removed ship still parses")
	}
	render()
}
