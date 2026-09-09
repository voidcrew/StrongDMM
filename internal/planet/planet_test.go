package planet

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/third_party/sdmmparser"
)

func fixture(t *testing.T) *Catalog {
	t.Helper()
	root := t.TempDir()
	source := `/datum/map_generator/planet_generator
	var/perlin_zoom = 65
	var/mountain_height = 0.85
	var/initial_closed_chance = 45
	var/smoothing_iterations = 20
	var/birth_limit = 4
	var/death_limit = 3
/turf/open/grass
/turf/open/sand
/turf/closed/rock
/obj/tree
/mob/living/test
/datum/biome
	var/open_turf_types = list(/turf/open/grass = 1)
	var/list/flora_spawn_list = list(/obj/tree = 1)
	var/list/feature_spawn_list
	var/list/mob_spawn_list = list(/mob/living/test = 1)
	var/list/dangerous_mob_spawn_list
	var/list/megafauna_spawn_list
	var/flora_spawn_chance = 2
	var/feature_spawn_chance = 0.1
	var/mob_spawn_chance = 6
/datum/biome/grass
/datum/biome/cave
	var/closed_turf_types = list(/turf/closed/rock = 1)
/datum/biome/cave/rock
/datum/planet
	var/list/overworld_biomes
	var/list/cave_biomes
/datum/planet/test
	// Keep my handwritten settings and comments.
	var/custom = 17
	overworld_biomes = SURFACE
	cave_biomes = CAVES
/datum/planet/other
	overworld_biomes = SURFACE
	cave_biomes = CAVES
/area/overmap_encounter/planetoid
	var/planet_type
	var/map_generator
/area/overmap_encounter/planetoid/test
	planet_type = /datum/planet/test
	map_generator = /datum/map_generator/planet_generator
/area/overmap_encounter/planetoid/cave
/datum/overmap/planet
	var/name
	var/planet_template
	var/mapgen
	var/target_area
	var/surface_area
/datum/overmap/planet/test
	name = "Test planet"
	planet_template = /datum/planet/test
	mapgen = /datum/map_generator/planet_generator
	target_area = /area/overmap_encounter/planetoid/test
	surface_area = /area/overmap_encounter/planetoid/test
/obj/structure/overmap/planet
	var/planet
`
	grid := func(n int, path string) [][]string {
		out := make([][]string, n)
		for i := range out {
			out[i] = []string{path, path, path, path, path}
		}
		return out
	}
	source = strings.ReplaceAll(source, "SURFACE", renderClimate(grid(6, "/datum/biome/grass"), HeatKeys))
	source = strings.ReplaceAll(source, "CAVES", renderClimate(grid(4, "/datum/biome/cave/rock"), CaveKeys))
	if err := os.WriteFile(filepath.Join(root, "content.dm"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "test.dme"), []byte("#include \"content.dm\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(root, markerRegistry)
	if err := os.MkdirAll(filepath.Dir(registry), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(registry, []byte("/datum/controller/subsystem/overmap/proc/setup_planets()\n\tvar/list/dynamic_planet_markers = list(\n\t\t/obj/structure/overmap/planet/test,\n\t)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return load(t, filepath.Join(root, "test.dme"))
}
func load(t *testing.T, path string) *Catalog {
	t.Helper()
	d, e := dmenv.New(path)
	if e != nil {
		t.Fatal(e)
	}
	c, e := Discover(d)
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func definition(t *testing.T, c *Catalog, path string) Definition {
	t.Helper()
	for _, d := range c.Planets {
		if d.Path == path {
			return d
		}
	}
	t.Fatal("missing planet", path)
	return Definition{}
}

func TestTerrainDeterministicAndBiomeIsolation(t *testing.T) {
	c := fixture(t)
	s := NewState(c, definition(t, c, "/datum/planet/test"))
	s.Size = 32
	o := PreviewOptions{Populate: true}
	a, e := Generate(c, s, c.Dme, o)
	if e != nil {
		t.Fatal(e)
	}
	b, e := Generate(c, s, c.Dme, o)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(a.Cells, b.Cells) {
		t.Fatal("seed is not repeatable")
	}
	local := s.LocalBiome("/datum/biome/grass")
	edit := s.Biomes[local]
	edit.Tables[0].Entries = []Entry{{"/turf/open/sand", 1}}
	s.Biomes[local] = edit
	changed, e := Generate(c, s, c.Dme, o)
	if e != nil {
		t.Fatal(e)
	}
	for i, cell := range a.Cells {
		after := changed.Cells[i]
		if cell.Heat != after.Heat || cell.Height != after.Height || cell.Moisture != after.Moisture {
			t.Fatal("biome edit moved noise fields")
		}
		if cell.Biome == "/datum/biome/grass" && after.Turf != "/turf/open/sand" {
			t.Fatal("replacement not visible")
		}
	}
	if c.Biomes["/datum/biome/grass"].Tables[0].Entries[0].Path != "/turf/open/grass" {
		t.Fatal("mutated shared catalog")
	}
	if s.Biomes["/datum/biome/grass"].Tables[0].Entries[0].Path != "/turf/open/grass" {
		t.Fatal("local copy changed its starting biome")
	}
	other := NewState(c, definition(t, c, "/datum/planet/other"))
	if other.Definition.Surface[0][0] != "/datum/biome/grass" {
		t.Fatal("changed another planet")
	}
}
func TestExistingSaveReopenAndExternalEdit(t *testing.T) {
	for _, terrain := range []bool{false, true} {
		t.Run(map[bool]string{false: "biome", true: "terrain"}[terrain], func(t *testing.T) {
			c := fixture(t)
			p, e := Open(c, definition(t, c, "/datum/planet/test"))
			if e != nil {
				t.Fatal(e)
			}
			path := p.State.LocalBiome("/datum/biome/grass")
			b := p.State.Biomes[path]
			b.Name = "Meadow"
			b.Tables[0].Entries = []Entry{{"/turf/open/sand", 2}, {"/turf/open/grass", 1}}
			p.State.Biomes[path] = b
			if terrain {
				p.State.Definition.Settings.Zoom = 90
			}
			if e = p.Save(); e != nil {
				t.Fatal(e)
			}
			if p.Modified() {
				t.Fatal("saved planet still dirty")
			}
			raw, _ := os.ReadFile(filepath.Join(c.Dme.RootDir, "content.dm"))
			if !bytes.Contains(raw, []byte("// Keep my handwritten settings and comments.")) || !bytes.Contains(raw, []byte("var/custom = 17")) {
				t.Fatal("lost handwritten source")
			}
			c2 := load(t, c.Dme.RootFile)
			p2, e := Open(c2, definition(t, c2, "/datum/planet/test"))
			if e != nil {
				t.Fatal(e)
			}
			if p2.State.Biomes[path].Name != "Meadow" {
				t.Fatal("lost biome name")
			}
			if terrain && p2.State.Definition.Settings.Zoom != 90 {
				t.Fatal("lost terrain settings")
			}
			p2.State.Seeds.Heat++
			codeBefore, _ := os.ReadFile(p2.code)
			if e = os.WriteFile(p2.code, append(codeBefore, []byte("// external edit\n")...), 0600); e != nil {
				t.Fatal(e)
			}
			if e = p2.Save(); e == nil {
				t.Fatal("overwrote external edit")
			}
		})
	}
}
func TestNewPlanetSaveReopen(t *testing.T) {
	c := fixture(t)
	p, e := Create(c, definition(t, c, "/datum/planet/test"), "Glasswood", true)
	if e != nil {
		t.Fatal(e)
	}
	if e = p.Save(); e != nil {
		t.Fatal(e)
	}
	c2 := load(t, c.Dme.RootFile)
	d := definition(t, c2, p.State.Definition.Path)
	if d.Name != "Glasswood" || len(d.Caves) != 0 {
		t.Fatal("new planet registration did not load", d)
	}
	p2, e := Open(c2, d)
	if e != nil {
		t.Fatal(e)
	}
	p2.State.Seeds.Heat++
	if e = p2.Save(); e != nil {
		t.Fatal(e)
	}
}
func TestNoiseAndClimateBoundaries(t *testing.T) {
	values := sdmmparser.PlanetNoise(123, 0, 0, 1, 3, 3)
	for _, n := range values {
		if n != .5 {
			t.Fatal("Perlin integer lattice should normalize to .5", n)
		}
	}
	a := sdmmparser.PlanetNoise(123, .3, .7, .1, 3, 3)
	b := sdmmparser.PlanetNoise(321, .3, .7, .1, 3, 3)
	if reflect.DeepEqual(a, b) {
		t.Fatal("seed ignored")
	}
	if band(.6, .2, .4, .6, .65, .8) != 2 || band(.60001, .2, .4, .6, .65, .8) != 3 {
		t.Fatal("DM boundary order differs")
	}
}
func TestRealPlanetCatalog(t *testing.T) {
	path := os.Getenv("PLANET_TEST_DME")
	if path == "" {
		t.Skip("set PLANET_TEST_DME for project acceptance")
	}
	c := load(t, path)
	t.Logf("%d planets, %d biomes, %d unreadable biomes", len(c.Planets), len(c.Biomes), len(c.Errors))
	for _, e := range c.Errors {
		t.Log(e)
	}
	if len(c.Planets) < 5 {
		t.Fatal("missing project planets")
	}
	for _, d := range c.Planets {
		p, e := Open(c, d)
		if e != nil {
			t.Error(d.Path, e)
			continue
		}
		if e = p.State.Validate(c); e != nil {
			t.Error(d.Path, e)
			continue
		}
		preview, e := Generate(c, p.State, c.Dme, PreviewOptions{Populate: true})
		if e != nil {
			t.Error(d.Path, e)
		} else {
			t.Logf("%s: %d tiles, %d visible biomes", d.Name, len(preview.Cells), len(preview.Counts))
		}
	}
}
