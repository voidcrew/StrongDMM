package planet

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDeleteBiomeReplacesEveryClimateAndPreservesSharedSource(t *testing.T) {
	c := fixture(t)
	d := definition(t, c, "/datum/planet/test")
	p, err := Open(c, d)
	if err != nil {
		t.Fatal(err)
	}
	old := "/datum/biome/grass"
	replacement := p.State.LocalBiome(old)
	// Import a second surface biome without replacing all the old assignments.
	p.State.Definition = Clone(State{Definition: d}).Definition
	b := p.State.Biomes[replacement]
	b.Name = "Replacement sand"
	b.Tables[0].Entries = []Entry{{Path: "/turf/open/sand", Weight: 1}}
	p.State.Biomes[replacement] = b
	if err = p.DeleteBiome(old, replacement); err != nil {
		t.Fatal(err)
	}
	if n, _ := p.BiomeUse(old); n != 0 {
		t.Fatal("left old climate references")
	}
	if n, _ := p.BiomeUse(replacement); n != 30 {
		t.Fatal("did not replace all 30 surface cells", n)
	}
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	c2 := load(t, c.Dme.RootFile)
	p2, err := Open(c2, definition(t, c2, d.Path))
	if err != nil {
		t.Fatal(err)
	}
	if p2.State.Definition.Surface[0][0] != replacement {
		t.Fatal("deletion did not survive reopening")
	}
	if c2.Dme.Objects[old] == nil || definition(t, c2, "/datum/planet/other").Surface[0][0] != old {
		t.Fatal("deletion changed the shared biome or another planet")
	}
	preview, err := Generate(c2, p2.State, c2.Dme, PreviewOptions{Biome: replacement})
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range preview.Cells {
		if cell.Turf != "/turf/open/sand" {
			t.Fatal("replacement terrain is not visible")
		}
	}
}

func TestDeleteBiomeRejectsInvalidReplacementsWithoutMutation(t *testing.T) {
	c := fixture(t)
	p, err := Open(c, definition(t, c, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	before := Clone(p.State)
	for _, pair := range [][2]string{
		{"/datum/biome/grass", ""},
		{"/datum/biome/grass", "/datum/biome/grass"},
		{"/datum/biome/grass", "/datum/biome/missing"},
		{"/datum/biome/cave/rock", "/datum/biome/grass"},
		{"/datum/biome/missing", "/datum/biome/grass"},
	} {
		if err := p.DeleteBiome(pair[0], pair[1]); err == nil {
			t.Fatal("accepted invalid replacement", pair)
		}
		if !reflect.DeepEqual(before, p.State) {
			t.Fatal("failed deletion changed state", pair)
		}
	}
}

func TestDeleteSavedLocalBiomeAndReopen(t *testing.T) {
	c := fixture(t)
	p, err := Open(c, definition(t, c, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	local := p.State.LocalBiome("/datum/biome/grass")
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	c = load(t, c.Dme.RootFile)
	p, err = Open(c, definition(t, c, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	if err = p.DeleteBiome(local, "/datum/biome/grass"); err != nil {
		t.Fatal(err)
	}
	for _, b := range p.BiomeChoices(false) {
		if b.Path == local {
			t.Fatal("deleted local biome is still offered for assignment")
		}
	}
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(p.code)
	if bytes.Contains(raw, []byte(local)) {
		t.Fatal("deleted custom definition remains in generated DM")
	}
	c2 := load(t, c.Dme.RootFile)
	p2, err := Open(c2, definition(t, c2, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := p2.State.Biomes[local]; ok || c2.Dme.Objects[local] != nil {
		t.Fatal("deleted local definition returned on reopen")
	}
}

func TestDeleteUnassignedBiomePreservesClimate(t *testing.T) {
	c := fixture(t)
	p, err := Open(c, definition(t, c, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	before := Clone(p.State)
	local := p.State.LocalBiome("/datum/biome/grass")
	p.State.Definition = Clone(before).Definition
	if err = p.DeleteBiome(local, ""); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, p.State) {
		t.Fatal("deleting unassigned biome changed other planet data")
	}
}

func TestDeleteLocalBiomeProtectsExternalUsers(t *testing.T) {
	c := fixture(t)
	p, err := Open(c, definition(t, c, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	local := p.State.LocalBiome("/datum/biome/grass")
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(c.Dme.RootDir, "content.dm"), os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("\n/datum/biome/external_child\n\tparent_type = " + local + "\n")
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	c = load(t, c.Dme.RootFile)
	p, err = Open(c, definition(t, c, "/datum/planet/test"))
	if err != nil {
		t.Fatal(err)
	}
	before := Clone(p.State)
	if err = p.DeleteBiome(local, "/datum/biome/grass"); err == nil || !reflect.DeepEqual(before, p.State) {
		t.Fatal("deleted a definition needed by handwritten source")
	}
}
