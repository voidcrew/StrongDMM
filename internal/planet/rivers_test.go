package planet

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlanetRivers(t *testing.T) {
	c := fixture(t)
	source := filepath.Join(c.Dme.RootDir, "content.dm")
	f, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`
/datum/overmap/planet/test
	var/ruin_type = "Lava Ruins"
/turf/open/lava/smooth/lava_land_surface/planetary
/turf/open/lava/plasma/planetary
/turf/open/protected
	var/turf_flags = 8
`)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	c = load(t, c.Dme.RootFile)
	s := NewState(c, definition(t, c, "/datum/planet/test"))
	s.Size = 64
	base := Clone(s)
	base.Definition.Overmap, base.BaseOvermap = "", ""
	before, err := Generate(c, base, c.Dme, PreviewOptions{Populate: true})
	if err != nil {
		t.Fatal(err)
	}
	a, err := Generate(c, s, c.Dme, PreviewOptions{Populate: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(c, s, c.Dme, PreviewOptions{Populate: true})
	if err != nil || !reflect.DeepEqual(a.Cells, b.Cells) {
		t.Fatal("river generation must repeat exactly", err)
	}
	rivers := 0
	for i, cell := range a.Cells {
		if cell.Turf == "" || a.Map.Tiles[i].Instances()[0].Prefab().Path() != cell.Turf {
			t.Fatal("terrain inspection and map disagree", i, cell.Turf)
		}
		if cell.River {
			rivers++
			if cell.Turf != RiverTurf(c, s) || cell.Closed || cell.Spawn != "" {
				t.Fatal("river retained walls or biome population", cell)
			}
		}
		if cell.Heat != before.Cells[i].Heat || cell.Biome != before.Cells[i].Biome {
			t.Fatal("river pass altered underlying climate")
		}
	}
	if rivers < 50 || rivers > len(a.Cells)/2 {
		t.Fatal("unexpected river coverage", rivers)
	}
	s.Seeds.Detail++
	b, err = Generate(c, s, c.Dme, PreviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	different := false
	for i := range a.Cells {
		different = different || a.Cells[i].River != b.Cells[i].River
	}
	if !different {
		t.Fatal("reroll did not change river course")
	}
	for _, options := range []PreviewOptions{{Caves: true}, {Biome: "/datum/biome/grass"}} {
		p, err := Generate(c, s, c.Dme, options)
		if err != nil {
			t.Fatal(err)
		}
		for _, cell := range p.Cells {
			if cell.River {
				t.Fatal("surface rivers leaked into underground or biome-only sample")
			}
		}
	}
	// New drafts inherit their environment before their own overmap type exists.
	s.Definition.Overmap = "/datum/overmap/planet/unsaved_new"
	if RiverTurf(c, s) == "" {
		t.Fatal("new planet lost its environment's rivers")
	}
	s.Definition.Caves = nil
	for path, biome := range s.Biomes {
		biome.Tables[0].Entries = []Entry{{Path: "/turf/open/protected", Weight: 1}}
		s.Biomes[path] = biome
	}
	protected, err := Generate(c, s, c.Dme, PreviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range protected.Cells {
		if cell.River || cell.Turf != "/turf/open/protected" {
			t.Fatal("river overwrote NO_LAVA_GEN terrain")
		}
	}
}
