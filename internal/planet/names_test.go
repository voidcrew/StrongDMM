package planet

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedFilesUsePlanetName(t *testing.T) {
	c := fixture(t)
	p, e := Create(c, definition(t, c, "/datum/planet/test"), "Glasswood", true)
	if e != nil {
		t.Fatal(e)
	}
	if p.State.Definition.Path != "/datum/planet/workshop_glasswood" || filepath.Base(p.GeneratedPath()) != "glasswood.dm" {
		t.Fatal("generated file carries the internal typepath prefix", p.GeneratedPath())
	}
	if e = p.Save(); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(filepath.Dir(p.GeneratedPath()), "glasswood.json")); e != nil {
		t.Fatal("metadata name differs from the source name", e)
	}
	dme, _ := os.ReadFile(c.Dme.RootFile)
	if !bytes.Contains(dme, []byte(`#include "voidcrew\mapping\planet_projects\glasswood.dm"`)) || bytes.Contains(dme, []byte("workshop_")) || bytes.Contains(dme, []byte("planet_projects/")) {
		t.Fatalf("registration uses the wrong name or slashes:\n%s", dme)
	}
	c2 := load(t, c.Dme.RootFile)
	if _, e = Open(c2, definition(t, c2, p.State.Definition.Path)); e != nil {
		t.Fatal(e)
	}
	// A handwritten planet's identifier is reserved even when its name differs.
	if _, e = Create(c2, definition(t, c2, "/datum/planet/test"), "OTHER!", true); e == nil {
		t.Fatal("generated files could shadow a handwritten planet")
	}
	if _, e = Create(c2, definition(t, c2, "/datum/planet/test"), "glasswood", true); e == nil {
		t.Fatal("duplicate planet accepted")
	}
}

func TestLegacyGeneratedFilesStayReadable(t *testing.T) {
	c := fixture(t)
	p, e := Create(c, definition(t, c, "/datum/planet/test"), "Glasswood", true)
	if e != nil {
		t.Fatal(e)
	}
	p.State.Seeds.Heat = 777
	if e = p.Save(); e != nil {
		t.Fatal(e)
	}
	// Older workshop versions wrote workshop_<id> files and registrations.
	dir := filepath.Dir(p.GeneratedPath())
	for _, ext := range []string{".dm", ".json"} {
		if e = os.Rename(filepath.Join(dir, "glasswood"+ext), filepath.Join(dir, "workshop_glasswood"+ext)); e != nil {
			t.Fatal(e)
		}
	}
	dme, _ := os.ReadFile(c.Dme.RootFile)
	dme = bytes.ReplaceAll(dme, []byte(`planet_projects\glasswood.dm`), []byte(`planet_projects/workshop_glasswood.dm`))
	if e = os.WriteFile(c.Dme.RootFile, dme, 0600); e != nil {
		t.Fatal(e)
	}
	c2 := load(t, c.Dme.RootFile)
	p2, e := Open(c2, definition(t, c2, p.State.Definition.Path))
	if e != nil {
		t.Fatal(e)
	}
	if filepath.Base(p2.GeneratedPath()) != "workshop_glasswood.dm" || p2.State.Seeds.Heat != 777 || p2.Modified() {
		t.Fatal("legacy project did not open from its own files", p2.GeneratedPath(), p2.State.Seeds.Heat, p2.Modified())
	}
	p2.State.Seeds.Heat++
	if e = p2.Save(); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(dir, "glasswood.json")); !os.IsNotExist(e) {
		t.Fatal("legacy project was duplicated under the new name")
	}
	dme, _ = os.ReadFile(c.Dme.RootFile)
	if bytes.Count(dme, []byte("planet_projects")) != 1 || !bytes.Contains(dme, []byte(`#include "voidcrew\mapping\planet_projects\workshop_glasswood.dm"`)) {
		t.Fatalf("legacy registration was duplicated or left unnormalized:\n%s", dme)
	}
	c3 := load(t, c.Dme.RootFile)
	p3, e := Open(c3, definition(t, c3, p.State.Definition.Path))
	if e != nil {
		t.Fatal(e)
	}
	if p3.State.Seeds.Heat != 778 {
		t.Fatal("legacy save was lost", p3.State.Seeds.Heat)
	}
}
