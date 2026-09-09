package wsruin

import (
	"testing"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/planet"
)

func testPlanetRuinPreview(t *testing.T, ws *WsRuin, capture func(string)) {
	t.Helper()
	found := false
	for _, item := range ws.catalog.Templates {
		if item.Location != "Wasteland planet" || item.Problem != "" {
			continue
		}
		ws.selectRuin(item)
		if !ws.hasPlanetDestination() {
			t.Fatal("planet ruin has no View on planet action")
		}
		ws.beginPlanetView()
		capture("ruin-on-planet")
		if ws.planetPreview == nil || ws.message != "" {
			t.Fatal("planet preview failed", ws.message)
		}
		live, err := ws.loadPreviewRuin()
		if err != nil {
			t.Fatal(err)
		}
		live.Name = "Unsaved ruin edits"
		ws.LiveMap = func(string) *dmmap.Dmm { return live }
		copy, err := ws.loadPreviewRuin()
		if err != nil || copy == live || copy.Name != live.Name {
			t.Fatal("preview did not take an isolated snapshot of unsaved map edits")
		}
		states := ws.hostPlanets()
		draft := planet.Clone(states[0])
		draft.Definition.Name = "Unsaved planet draft"
		ws.PlanetDrafts = func() []planet.State { return []planet.State{draft} }
		if ws.hostPlanets()[0].Definition.Name != draft.Definition.Name {
			t.Fatal("preview did not use unsaved planet edits")
		}
		ws.planetRefresh = true
		capture("ruin-on-planet-unsaved")
		ws.LiveMap, ws.PlanetDrafts = nil, nil
		found = true
		break
	}
	if !found {
		t.Fatal("no planet ruin tested")
	}
	ws.viewOnPlanet = false
	for _, item := range ws.catalog.Templates {
		if item.Location == "Space" {
			ws.selectRuin(item)
			if ws.hasPlanetDestination() {
				t.Fatal("space-only ruin offered a planet preview")
			}
			break
		}
	}
}
