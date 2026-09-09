package planet

import (
	"reflect"
	"testing"
)

func TestDiscardBiomeChangesIsScopedAndRestoresSavedContent(t *testing.T) {
	c := fixture(t)
	p, err := Open(c, definition(t, c, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	before := Clone(p.State)
	local := p.State.LocalBiome("/datum/biome/grass")
	b := p.State.Biomes[local]
	b.Name, b.Tables[0].Entries[0].Path = "Changed", "/turf/open/sand"
	p.State.Biomes[local] = b
	p.State.Seeds.Heat++
	if !p.CanDiscardBiome(local) {
		t.Fatal("edited biome has no discard action")
	}
	path, err := p.DiscardBiomeChanges(local)
	if err != nil || path != "/datum/biome/grass" {
		t.Fatal(path, err)
	}
	before.Seeds.Heat++
	if !reflect.DeepEqual(before, p.State) {
		t.Fatal("discarding biome lost unrelated edits or left a local copy")
	}
	local = p.State.LocalBiome(path)
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	b = p.State.Biomes[local]
	b.Name = "Another edit"
	p.State.Biomes[local] = b
	if path, err = p.DiscardBiomeChanges(local); err != nil || path != local {
		t.Fatal("saved custom biome lost identity", path, err)
	}
	if p.Modified() {
		t.Fatal("discarded saved biome is still modified")
	}
}

func TestDiscardNewPlanetBiomeReturnsToStartingContents(t *testing.T) {
	c := fixture(t)
	p, err := Create(c, definition(t, c, "/datum/planet/test"), "Draft", true)
	if err != nil {
		t.Fatal(err)
	}
	before := Clone(p.State)
	path := p.State.UsedBiomes()[0]
	b := p.State.Biomes[path]
	b.Name = "Changed ground"
	p.State.Biomes[path] = b
	if _, err = p.DiscardBiomeChanges(path); err != nil || !reflect.DeepEqual(before, p.State) {
		t.Fatal("did not restore new planet's starting biome", err)
	}
}

func TestDiscardNewBiomeRestoresItsStartingContents(t *testing.T) {
	c := fixture(t)
	p, err := Open(c, definition(t, c, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	b := Clone(p.State).Biomes["/datum/biome/grass"]
	b.Path, b.Name, b.Local, b.Created = "/datum/biome/workshop_new", "New biome", true, true
	p.State.Biomes[b.Path] = b
	p.RememberNewBiome(b.Path)
	b.Tables[0].Entries[0].Path = "/turf/open/sand"
	b.Name = "Edited new biome"
	p.State.Biomes[b.Path] = b
	if !p.CanDiscardBiome(b.Path) {
		t.Fatal("new biome edits cannot be discarded")
	}
	path, err := p.DiscardBiomeChanges(b.Path)
	if err != nil || path != b.Path || p.State.Biomes[path].Name != "New biome" || p.State.Biomes[path].Tables[0].Entries[0].Path != "/turf/open/grass" {
		t.Fatal("starting biome content was lost", err)
	}
}
