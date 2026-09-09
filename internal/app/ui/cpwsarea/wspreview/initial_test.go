package wspreview

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/mappreview"
	"sdmm/internal/ship"
	"sdmm/internal/util"
)

// Compare actual rendered pixels: a fan is visible through an open airlock,
// but adding it below a solid closed door must not change the image.
func exerciseDoorOcclusion(t *testing.T, dme *dmenv.Dme) {
	t.Helper()
	for _, mode := range []string{"closed", "open"} {
		m := &dmmap.Dmm{Name: "door-" + mode, MaxX: 1, MaxY: 1, MaxZ: 1}
		tile := &dmmap.Tile{Coord: util.Point{X: 1, Y: 1, Z: 1}}
		m.Tiles = []*dmmap.Tile{tile}
		for _, path := range []string{"/turf/open/floor/iron", "/obj/structure/fans/tiny", "/obj/machinery/door/airlock"} {
			object := dme.Objects[path]
			if object == nil {
				t.Fatalf("missing door fixture type %s", path)
			}
			vars := dmvars.FromParent(object.Vars)
			if path == "/obj/machinery/door/airlock" {
				if mode == "open" {
					vars = dmvars.Set(vars, "density", "0")
				}
			}
			tile.InstancesAdd(dmmprefab.New(dmmprefab.IdStage, path, vars))
		}
		capture := func() []byte {
			t.Helper()
			p := New(mappreview.Snapshot(m, 1), dme)
			defer p.Dispose()
			p.options.Lighting = false
			p.rebuild()
			path := filepath.Join(t.TempDir(), "door.png")
			if err := p.exportTo(path); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
		withFan := capture()
		items := tile.Instances()
		tile.Set(dmmap.Instances{items[0], items[2]})
		withoutFan := capture()
		if hidden := bytes.Equal(withFan, withoutFan); hidden != (mode == "closed") {
			t.Fatalf("%s airlock: fan hidden=%v", mode, hidden)
		}
	}
}

func exerciseAssembledPreview(t *testing.T, dme *dmenv.Dme, output string) {
	t.Helper()
	request := os.Getenv("MAP_PREVIEW_TEST_SHIP")
	if request == "" {
		return
	}
	name, themeID, _ := strings.Cut(request, "/")
	catalog, err := ship.Discover(dme)
	if err != nil {
		t.Fatal(err)
	}
	for _, hull := range catalog.Hulls {
		if hull.Type != ship.HullType+"/"+name {
			continue
		}
		for _, theme := range hull.Themes {
			if theme.ID != themeID {
				continue
			}
			selected := map[string]string{}
			for _, module := range hull.Modules {
				if module.Default && module.Available(theme.ID) {
					selected[module.Slot] = module.ID
				}
			}
			assembly, err := catalog.Load(hull, theme, selected)
			if err != nil {
				t.Fatal(err)
			}
			originals := map[string][]byte{}
			for _, source := range assembly.Sources {
				originals[source.File], err = os.ReadFile(source.File)
				if err != nil {
					t.Fatal(err)
				}
			}
			m, err := assembly.Display(dme)
			if err != nil {
				t.Fatal(err)
			}
			p := New(mappreview.Snapshot(m, 1), dme)
			defer p.Dispose()
			if p.message != "" {
				t.Fatalf("assembled preview: %s %v", p.message, p.scene.Notes)
			}
			for _, note := range p.scene.Notes {
				t.Logf("assembled preview: %s", note)
				if strings.Contains(note, "Window spawner") || strings.Contains(note, "shuttle_window") || strings.Contains(note, "dollhouse_wall") {
					t.Fatalf("ship hull appearance incomplete: %s", note)
				}
			}
			if err := p.exportTo(filepath.Join(output, "preview-assembled-lit.png")); err != nil {
				t.Fatal(err)
			}
			p.options.Lighting = false
			p.rebuild()
			if err := p.exportTo(filepath.Join(output, "preview-assembled-smooth.png")); err != nil {
				t.Fatal(err)
			}
			for path, before := range originals {
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("preview changed ship source %s", path)
				}
			}
			return
		}
	}
	t.Fatalf("assembled preview fixture not found: %s", request)
}
