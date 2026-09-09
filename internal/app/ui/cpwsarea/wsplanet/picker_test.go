package wsplanet

import (
	"strings"
	"testing"

	"sdmm/internal/planet"
)

func testCavePicker(t *testing.T, w *Workspace, capture func(string)) {
	t.Helper()
	p := w.project
	saved := planet.Clone(p.State)
	defer func() { p.State = saved; w.bind(p) }()
	for _, path := range p.State.UsedBiomes() {
		b := p.State.Biomes[path]
		if !b.Cave {
			continue
		}
		w.selectBiome(path)
		for i, table := range b.Tables {
			if table.Field == "closed_turf_types" {
				w.table = i
			}
		}
		break
	}
	w.mode, w.picking, w.replace, w.filter, w.pickerScope = 1, true, 0, "", 0
	items := w.pickerItems("closed_turf_types")
	if len(items) < 5 {
		t.Fatal("missing recommended cave walls", items)
	}
	names := map[string]bool{}
	for _, path := range items {
		name := w.itemName(path)
		if strings.Contains(path, "/debug") || name == "voidcrew" || names[name] {
			t.Error("ambiguous or debug cave choice", name, path)
		}
		names[name] = true
		if w.thumbnail(path) == nil {
			t.Error("cave wall has no thumbnail", path)
		}
		t.Log(name, path)
	}
	capture("cave-wall-picker")
	w.pickerScope = 2
	capture("built-wall-picker")
	w.pickerScope = 0
	w.chooseItem("/turf/closed/mineral/random/snow")
	capture("cave-wall-replaced")
	if w.picking || w.selectedEntry != "/turf/closed/mineral/random/snow" {
		t.Fatal("cave replacement did not select the changed entry")
	}
}
