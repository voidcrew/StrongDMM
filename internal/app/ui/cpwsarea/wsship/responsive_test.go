package wsship

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Exercise the actual nested native windows at each layout transition. Changing
// views and resizing must preserve the selected roster, item slot and equipment.
func exerciseResponsiveCrew(t *testing.T, ws *WsShip, render func(), resize func(int, int)) {
	t.Helper()
	ws.setStage(stepBuild)
	ws.beginCrew()
	for _, scope := range ws.crew.scopes {
		if len(ws.crew.jobs) > 0 {
			break
		}
		ws.loadCrewScope(scope.ID)
	}
	if ws.crew.error != "" || len(ws.crew.jobs) == 0 {
		t.Fatalf("crew unavailable: %s", ws.crew.error)
	}
	selected := ws.crew.selected
	outfit := ws.project.CrewOutfit(ws.crew.jobs[selected])
	for _, size := range [][2]int{{1400, 960}, {1100, 760}, {860, 700}} {
		resize(size[0], size[1])
		for _, view := range []string{"equipment", "carry", "picker", "settings"} {
			ws.crew.settings = view == "settings"
			ws.crew.picking = view == "picker"
			ws.crew.group = 0
			if view == "carry" {
				ws.crew.group = 2
			}
			for frame := 0; frame < 3; frame++ {
				render()
			}
			if ws.Map() != nil || ws.crew.selected != selected || ws.crew.slot != 0 || ws.project.CrewOutfit(ws.crew.jobs[selected]) != outfit {
				t.Fatal("resizing or changing crew views altered editing state")
			}
			if dst := os.Getenv("SHIP_RENDER_TEST_OUTPUT"); dst != "" {
				captureFrame(t, filepath.Join(dst, fmt.Sprintf("crew-%d-%s.png", size[0], view)), size[0], size[1])
			}
		}
	}
	ws.setStage(stepBuild)
}
