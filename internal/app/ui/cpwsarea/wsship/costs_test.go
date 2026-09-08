package wsship

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/ship"
)

func exerciseResponsiveCosts(t *testing.T, ws *WsShip, render func(), resize func(int, int)) {
	t.Helper()
	ws.setStage(stepBuild)
	started := time.Now()
	ws.beginCosts("ship")
	t.Logf("Loaded %d real ship costs in %s", len(ws.costs.entries), time.Since(started))
	priced := -1
	for i, entry := range ws.costs.entries {
		if entry.error != "" {
			t.Fatalf("read existing costs for %s: %s", entry.scope.Name, entry.error)
		}
		if entry.value.Total() > 0 {
			priced = i
		}
	}
	if priced < 0 {
		t.Fatal("existing ship has no priced components")
	}
	ws.loadCostEntry(priced)
	for _, size := range [][2]int{{1400, 960}, {1100, 760}, {860, 700}} {
		resize(size[0], size[1])
		for frame := 0; frame < 3; frame++ {
			render()
		}
		if ws.Map() != nil || ws.costs.dirty || ws.project.Modified() {
			t.Fatal("viewing existing costs changed the ship or exposed map tools")
		}
		if dst := os.Getenv("SHIP_RENDER_TEST_OUTPUT"); dst != "" {
			captureFrame(t, filepath.Join(dst, fmt.Sprintf("costs-%d.png", size[0])), size[0], size[1])
		}
	}
	ws.finishTask()
}

func exerciseCosts(t *testing.T, ws *WsShip, render func(), legacy bool) {
	t.Helper()
	ws.setStage(stepBuild)
	// In the handwritten phase, another editor instance just saved crew changes.
	// Reopen before editing prices to exercise those current source definitions.
	if legacy {
		fresh, err := dmenv.New(ws.project.Dme.RootFile)
		if err != nil {
			t.Fatal(err)
		}
		p, err := ship.OpenProject(ws.catalog, fresh, ws.project.Hull)
		if err != nil {
			t.Fatal(err)
		}
		ws.app.(*previewApp).dme = fresh
		ws.projects[p.Hull.Type] = p
		ws.rebuild()
	}
	want := ship.PartCosts{"combat": 4, "science": 8, "trade": 12, "misc": 2}
	if legacy {
		want["combat"] = 7
	}
	for _, scope := range []string{"ship", "theme/pirate", "module/medical"} {
		ws.beginCosts(scope)
		if ws.costs.error != "" || ws.costs.selected < 0 {
			t.Fatal(ws.costs.error)
		}
		before := copyCosts(ws.costs.values)
		ws.costs.values["combat"] = -1
		ws.costs.dirty = true
		if ws.commitCosts() {
			t.Fatal("negative costs accepted")
		}
		ws.costs.values = copyCosts(want)
		ws.costs.dirty = true
		if !ws.commitCosts() {
			t.Fatal(ws.costs.error)
		}
		ws.app.CommandStorage().Undo()
		if ws.task != taskCosts || !ws.costs.values.Equal(before) {
			t.Fatal("undo did not restore the visible part costs")
		}
		ws.app.CommandStorage().Redo()
		if !ws.costs.values.Equal(want) || ws.Map() != nil {
			t.Fatal("redo did not restore cost editing state")
		}
	}
	// Saving must commit a still-open form, including a zero price.
	ws.costs.values["misc"] = 0
	ws.costs.dirty = true
	if !ws.Save() {
		t.Fatal(ws.message)
	}
	fresh, err := dmenv.New(ws.project.Dme.RootFile)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := ship.OpenProject(ws.catalog, fresh, ws.project.Hull)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"ship", "theme/pirate", "module/medical"} {
		expected := copyCosts(want)
		if scope == "module/medical" {
			expected["misc"] = 0
		}
		got, err := reopened.PartCosts(scope)
		if err != nil || !got.Equal(expected) {
			t.Fatalf("reopened %s costs = %v, want %v: %v", scope, got, expected, err)
		}
	}
	if legacy {
		data, err := os.ReadFile(filepath.Join(ws.catalog.Root, "voidcrew/mapping/shuttles/workshop_fixture.dm"))
		if err != nil || !bytes.Contains(data, []byte("Handwritten ship with custom jobs and costs")) {
			t.Fatal("cost editing erased handwritten definitions", err)
		}
	}
	ws.setStage(stepBuild)
	ws.beginCosts("module/medical")
	for frame := 0; frame < 3; frame++ {
		render()
	}
	if dst := os.Getenv("SHIP_RENDER_TEST_OUTPUT"); dst != "" {
		captureFrame(t, filepath.Join(dst, fmt.Sprintf("costs-fixture-legacy-%t.png", legacy)), 1400, 960)
	}
	ws.finishTask()
}
