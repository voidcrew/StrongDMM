package planet

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Materialize changed sources in a separate directory for optional BYOND
// acceptance. The real project's sources and DME are read only.
func TestExportRealPlanetDefinitions(t *testing.T) {
	path, out := os.Getenv("PLANET_TEST_DME"), os.Getenv("PLANET_TEST_EXPORT")
	if path == "" || out == "" {
		t.Skip("set PLANET_TEST_DME and PLANET_TEST_EXPORT to compile exported definitions")
	}
	c := load(t, path)
	existing, e := Open(c, definition(t, c, "/datum/planet/beach"))
	if e != nil {
		t.Fatal(e)
	}
	local := existing.State.LocalBiome("/datum/biome/grass")
	b := existing.State.Biomes[local]
	b.Name = "Glasswood grass"
	b.Tables[0].Entries[0].Weight = 2.5
	existing.State.Biomes[local] = b
	existing.State.Definition.Settings.Zoom = 85.25
	if d := &existing.State.Definition; d.Environment != nil {
		d.Environment.LightColor, d.Environment.Gravity = "#A4D8FF", 1.25
		d.Rivers.Enabled, d.Rivers.Turf, d.Rivers.Nodes = true, "/turf/open/water/beach", 6
		if c.Dme.Objects[d.Rivers.Turf] == nil {
			d.Rivers.Turf = d.Environment.Baseturf
		}
		d.Rivers.Biomes = []string{local}
		d.Ruins.Templates = []string{}
	}
	newPlanet, e := Create(c, definition(t, c, "/datum/planet/jungle"), "Glasswood acceptance", true)
	if e != nil {
		t.Fatal(e)
	}
	base, _ := os.ReadFile(path)
	replacements := map[string]string{}
	var includes []string
	for _, p := range []*Project{existing, newPlanet} {
		changes, e := p.Changes()
		if e != nil {
			t.Fatal(e)
		}
		for _, change := range changes {
			if strings.EqualFold(change.Path, c.Dme.RootFile) {
				continue
			}
			rel, e := filepath.Rel(c.Dme.RootDir, change.Path)
			if e != nil {
				t.Fatal(e)
			}
			dest := filepath.Join(out, rel)
			if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(dest, change.After, 0600); e != nil {
				t.Fatal(e)
			}
			replacements[strings.ToLower(filepath.Clean(change.Path))] = dest
		}
		includes = append(includes, filepath.Join(out, strings.TrimPrefix(p.code, c.Dme.RootDir+string(filepath.Separator))))
	}
	re := regexp.MustCompile(`(?m)^#include\s+"([^"\r\n]+)"`)
	dme := re.ReplaceAllStringFunc(string(base), func(line string) string {
		match := re.FindStringSubmatch(line)
		original := filepath.Join(c.Dme.RootDir, strings.ReplaceAll(match[1], "\\", "/"))
		if strings.EqualFold(filepath.Ext(original), ".dmf") {
			// BYOND's interface loader expects a local relative include even
			// when source DM includes are absolute in an acceptance project.
			contents, err := os.ReadFile(original)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(out, "skin.dmf"), contents, 0600); err != nil {
				t.Fatal(err)
			}
			return `#include "skin.dmf"`
		}
		dest := original
		if p, ok := replacements[strings.ToLower(filepath.Clean(original))]; ok {
			dest = p
		}
		return `#include "` + filepath.ToSlash(dest) + `"`
	})
	dme = strings.ReplaceAll(dme, "#define FILE_DIR .", `#define FILE_DIR "`+filepath.ToSlash(c.Dme.RootDir)+`"`)
	for _, inc := range includes {
		dme += "\n#include \"" + filepath.ToSlash(inc) + "\"\n"
	}
	dmePath := filepath.Join(out, "planet-acceptance.dme")
	if e = os.WriteFile(dmePath, []byte(dme), 0600); e != nil {
		t.Fatal(e)
	}
	loaded := load(t, dmePath)
	if len(loaded.Planets) != len(c.Planets)+1 {
		t.Fatal("exported planet registration not found")
	}
	t.Log(dmePath)
}
