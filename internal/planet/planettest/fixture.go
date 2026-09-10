// Package planettest writes a small Voidcrew-style environment with two
// planets for tests in the planet packages and the Planet Workshop UI.
package planettest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Registry is the project-relative source holding the planet spawn list.
const Registry = "voidcrew/modules/overmap/code/controllers/subsystem/overmap.dm"

var heat = []string{"coldest", "cold", "warm", "perfect", "hot", "hottest"}
var cave = []string{"coldest_cave", "cold_cave", "warm_cave", "hot_cave"}
var moisture = []string{"lowest_humidity", "low_humidity", "medium_humidity", "high_humidity", "highest_humidity"}

const source = `/datum/map_generator/planet_generator
	var/perlin_zoom = 65
	var/mountain_height = 0.85
	var/initial_closed_chance = 45
	var/smoothing_iterations = 20
	var/birth_limit = 4
	var/death_limit = 3
	var/list/heat_shares = list(20, 20, 20, 5, 15, 20)
	var/list/cave_heat_shares = list(25, 25, 25, 25)
	var/list/humidity_shares = list(20, 20, 20, 20, 20)
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

// Write creates the environment in a temporary directory and returns its DME.
func Write(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	text := strings.ReplaceAll(source, "SURFACE", climate(heat, "/datum/biome/grass"))
	text = strings.ReplaceAll(text, "CAVES", climate(cave, "/datum/biome/cave/rock"))
	write(t, filepath.Join(root, "content.dm"), text)
	write(t, filepath.Join(root, "test.dme"), "#include \"content.dm\"\n")
	write(t, filepath.Join(root, filepath.FromSlash(Registry)), "/datum/controller/subsystem/overmap/proc/setup_planets()\n\tvar/list/dynamic_planet_markers = list(\n\t\t/obj/structure/overmap/planet/test,\n\t)\n")
	return filepath.Join(root, "test.dme")
}

func write(t testing.TB, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func climate(keys []string, path string) string {
	var rows []string
	for _, key := range keys {
		var cells []string
		for _, m := range moisture {
			cells = append(cells, "\t\t\t\""+m+"\" = "+path)
		}
		rows = append(rows, "\t\t\""+key+"\" = list(\n"+strings.Join(cells, ",\n")+"\n\t\t)")
	}
	return "list(\n" + strings.Join(rows, ",\n") + "\n\t)"
}
