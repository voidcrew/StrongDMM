package planet

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/mappreview"
)

func settingsFixture(t *testing.T) *Catalog {
	c := fixture(t)
	file := filepath.Join(c.Dme.RootDir, "content.dm")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte(`
/datum/planet_environment
	var/area_type = /area/overmap_encounter/planetoid/test
	var/baseturf = /turf/open/grass
	var/atmosphere
	var/weather_type
	var/weather_trait
	var/light_color = "#FFFFFF"
	var/light_alpha = 255
	var/gravity = 1
/datum/planet_ruins
	var/enabled = 1
	var/theme = "Lava Ruins"
	var/budget_multiplier = 1
	var/mineral_budget = 15
	var/templates
/datum/planet_rivers
	var/enabled = 1
	var/turf_type = /turf/open/sand
	var/node_count = 4
	var/spread_chance = 25
	var/spread_loss = 11
	var/detour_chance = 20
	var/biomes
/datum/map_template/ruin
	var/id
	var/ruin_type
/datum/map_template/ruin/test
	id = "test"
	ruin_type = "Lava Ruins"
/datum/map_template/ruin/space
	id = "space"
	ruin_type = "Space Ruins"
`)...)
	data = []byte(strings.Replace(string(data), "/datum/planet\n", `/datum/planet
	var/definition_version = 1
	var/planet_size = 123
	var/terrain_generator = /datum/map_generator/planet_generator
	var/environment = /datum/planet_environment
	var/ruin_settings = /datum/planet_ruins
	var/river_settings = /datum/planet_rivers
`, 1))
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	return load(t, c.Dme.RootFile)
}

func TestPlanetSettingsRoundTrip(t *testing.T) {
	c := settingsFixture(t)
	p, err := Open(c, c.Planets[0])
	if err != nil {
		t.Fatal(err)
	}
	if p.State.Definition.Schema != 1 || p.State.Definition.Environment == nil || p.State.Definition.Ruins == nil {
		t.Fatal("new definition not read")
	}
	original := Clone(p.State)
	p.State.Definition.Environment.Gravity = 1.25
	p.State.Definition.Environment.LightColor = "#AABBCC"
	air := "TEMP=2.7"
	p.State.Definition.Environment.Atmosphere = &air
	p.State.Definition.Ruins.Templates = []string{}
	p.State.Definition.Ruins.Budget = 2
	p.State.Definition.Rivers.Biomes = []string{}
	p.State.Definition.Rivers.Nodes = 8
	p.State.Definition.Settings.Zoom = 99
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(c.Dme.RootDir, "content.dm"))
	if !strings.Contains(string(data), "terrain_generator = "+p.generatorPath()) || !strings.Contains(string(data), "var/gravity = 1") {
		t.Fatal("definition save did not isolate shared presets")
	}
	c = load(t, c.Dme.RootFile)
	q, err := Open(c, c.Planets[0])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.State.Definition, q.State.Definition) {
		t.Fatalf("settings changed on reopen\n%+v\n%+v", p.State.Definition, q.State.Definition)
	}
	if q.Modified() {
		t.Fatal("reopened project dirty")
	}
	if q.State.Definition.Ruins.Templates == nil || q.State.Definition.Rivers.Biomes == nil {
		t.Fatal("explicit empty selection became all")
	}
	if c.Planets[1].Environment.Gravity != original.Definition.Environment.Gravity || c.Planets[1].Rivers.Nodes != 4 {
		t.Fatal("another planet inherited private edits")
	}
	q.State.Definition.Rivers.Enabled = false
	q.State.Definition.Ruins.Templates = []string{"/datum/map_template/ruin/test"}
	if err := q.Save(); err != nil {
		t.Fatal(err)
	}
	c = load(t, c.Dme.RootFile)
	if _, err := Open(c, c.Planets[0]); err != nil {
		t.Fatal(err)
	}
}

func TestNewPlanetOwnsEnvironment(t *testing.T) {
	c := settingsFixture(t)
	base := c.Planets[0]
	for _, d := range c.Planets {
		if d.Overmap != "" {
			base = d
			break
		}
	}
	p, err := Create(c, base, "New world", false)
	if err != nil {
		t.Fatal(err)
	}
	if p.State.Definition.Environment.Area != p.State.Definition.Area {
		t.Fatal("new planet uses its parent's area")
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	c = load(t, c.Dme.RootFile)
	var found bool
	for _, d := range c.Planets {
		if d.Path == p.State.Definition.Path {
			found = true
			q, err := Open(c, d)
			if err != nil {
				t.Fatal(err)
			}
			if q.State.Definition.Area != p.State.Definition.Area || q.State.Definition.Generator != p.generatorPath() {
				t.Fatal("new planet definition did not own its generator/area")
			}
			q.State.Definition.Environment.Gravity = 2
			if err := q.Save(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !found {
		t.Fatal("saved planet missing")
	}
}

func TestRiverFiltersFollowBiomeEdits(t *testing.T) {
	c := settingsFixture(t)
	p, err := Open(c, c.Planets[0])
	if err != nil {
		t.Fatal(err)
	}
	old := p.State.UsedBiomes()[0]
	p.State.Definition.Rivers.Biomes = []string{old}
	next := p.State.LocalBiome(old)
	if !reflect.DeepEqual(p.State.Definition.Rivers.Biomes, []string{next}) {
		t.Fatal("local biome lost river eligibility")
	}
	if err := p.State.Validate(c); err != nil {
		t.Fatal(err)
	}
	p.State.Definition.Rivers.Nodes = 1
	if err := p.State.Validate(c); err == nil {
		t.Fatal("invalid river count accepted")
	}
	p.State.Definition.Rivers.Nodes = 4
	p.State.Definition.Ruins.Templates = []string{"/datum/map_template/ruin/space"}
	if err := p.State.Validate(c); err == nil {
		t.Fatal("space ruin accepted as a planet ruin")
	}
}

func TestPlanetSettingsProtectHandwrittenTypesAndBlankDraft(t *testing.T) {
	c := settingsFixture(t)
	file := filepath.Join(c.Dme.RootDir, "content.dm")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n/datum/planet_rivers/workshop_test\n\tnode_count = 7\n")...)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	c = load(t, c.Dme.RootFile)
	d := definition(t, c, "/datum/planet/test")
	p, err := Open(c, d)
	if err != nil {
		t.Fatal(err)
	}
	p.State.Definition.Rivers.Nodes = 8
	if _, err := p.Changes(); err == nil || !strings.Contains(err.Error(), "already defined outside") {
		t.Fatalf("handwritten river settings not protected: %v", err)
	}
	unchanged, _ := os.ReadFile(file)
	if string(unchanged) != string(data) {
		t.Fatal("failed save changed source")
	}
	blank, err := Create(c, d, "Empty world", true)
	if err != nil {
		t.Fatal(err)
	}
	if blank.State.Definition.Rivers.Enabled || blank.State.Definition.Ruins.Enabled {
		t.Fatal("empty draft inherited rivers or ruins")
	}
	if !d.Rivers.Enabled || !d.Ruins.Enabled {
		t.Fatal("empty draft mutated its starting planet")
	}
}

func TestPlanetEnvironmentLightsPreview(t *testing.T) {
	c := settingsFixture(t)
	s := NewState(c, c.Planets[0])
	s.Size = 24
	s.Definition.Settings.Mountain = 1
	s.Definition.Rivers.Enabled = false
	s.Definition.Environment.LightColor = "#FF0000"
	s.Definition.Environment.LightAlpha = 128
	dmmap.PrefabStorage.Free()
	dmmap.Init(c.Dme)
	p, err := Generate(c, s, c.Dme, PreviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	scene := mappreview.Build(p.Map, c.Dme, mappreview.Options{Lighting: true}, nil)
	light := scene.Lighting.Tile(12, 12)[0]
	if light[0] < .49 || light[0] > .51 || light[1] != 0 || light[2] != 0 {
		t.Fatalf("daylight ignored: %v", light)
	}
	if !c.AcceptsRuin(s, "/datum/map_template/ruin/test") {
		t.Fatal("theme ruin missing")
	}
	// Ordinary map previews also resolve the saved planet's daylight without
	// requiring Planet Workshop to construct the area prefab first.
	plain := Clone(s)
	plain.Definition.Environment = nil
	plain.Definition.Schema = 0
	mapOnly, err := Generate(c, plain, c.Dme, PreviewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	savedScene := mappreview.Build(mapOnly.Map, c.Dme, mappreview.Options{Lighting: true}, nil)
	if saved := savedScene.Lighting.Tile(12, 12)[0]; saved[0] != 1 || saved[1] != 1 || saved[2] != 1 {
		t.Fatalf("mapped planet did not inherit saved daylight: %v", saved)
	}
	s.Definition.Ruins.Templates = []string{}
	if c.AcceptsRuin(s, "/datum/map_template/ruin/test") {
		t.Fatal("disabled template still offered")
	}
}
