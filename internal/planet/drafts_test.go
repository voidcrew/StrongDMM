package planet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMultipleOpenPlanetsSaveSharedSources(t *testing.T) {
	for _, newPlanets := range []bool{false, true} {
		t.Run(map[bool]string{true: "new", false: "existing"}[newPlanets], func(t *testing.T) {
			c := fixture(t)
			var projects []*Project
			for i, path := range []string{"/datum/planet/test", "/datum/planet/other"} {
				var p *Project
				var err error
				if newPlanets {
					p, err = Create(c, definition(t, c, "/datum/planet/test"), []string{"First draft", "Second draft"}[i], true)
				} else {
					p, err = Open(c, definition(t, c, path))
				}
				if err != nil {
					t.Fatal(err)
				}
				if p.UnsavedNew() != newPlanets {
					t.Fatal("incorrect unsaved draft status")
				}
				local := p.State.LocalBiome(p.State.Definition.Surface[0][0])
				b := p.State.Biomes[local]
				b.Tables[0].Entries = []Entry{{Path: "/turf/open/sand", Weight: 1}}
				p.State.Biomes[local] = b
				projects = append(projects, p)
			}
			for _, p := range projects {
				changes, err := p.Changes()
				if err != nil {
					t.Fatal(err)
				}
				if err = p.Save(); err != nil {
					t.Fatal(err)
				}
				if p.UnsavedNew() {
					t.Fatal("saved planet still offered for cancellation")
				}
				c = load(t, c.Dme.RootFile)
				for _, other := range projects {
					if other != p {
						other.ObserveSave(changes, c)
					}
				}
			}
			for _, p := range projects {
				reopened, err := Open(c, definition(t, c, p.State.Definition.Path))
				if err != nil {
					t.Fatal(err)
				}
				b := reopened.State.Biomes[reopened.State.Definition.Surface[0][0]]
				if b.Tables[0].Entries[0].Path != "/turf/open/sand" {
					t.Fatal("lost another planet's changes")
				}
			}
			// External edits still stop a subsequent save.
			file := filepath.Join(c.Dme.RootDir, "content.dm")
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(file, append(data, []byte("\n// external change\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			if !newPlanets {
				projects[0].State.Seeds.Heat++
				if err = projects[0].Save(); err == nil {
					t.Fatal("overwrote external changes")
				}
			}
		})
	}
}
