package planet

import (
	"reflect"
	"testing"
)

func TestBiomeOrderFollowsTheMapper(t *testing.T) {
	c := fixture(t)
	p, e := Open(c, definition(t, c, "/datum/planet/test"))
	if e != nil {
		t.Fatal(e)
	}
	s := &p.State
	grass, rock := "/datum/biome/grass", "/datum/biome/cave/rock"
	if got := s.VisibleBiomes(); !reflect.DeepEqual(got, []string{grass, rock}) {
		t.Fatal("unexpected default order", got)
	}
	if s.MoveBiome(rock, 1) || s.MoveBiome(grass, -1) || s.MoveBiome("/datum/biome/missing", 1) || s.MoveBiome(grass, 0) {
		t.Fatal("moved outside the list")
	}
	if !s.MoveBiome(rock, -1) || !reflect.DeepEqual(s.VisibleBiomes(), []string{rock, grass}) || !p.Modified() {
		t.Fatal("reorder was not applied", s.VisibleBiomes())
	}
	// Renaming makes a local copy; it keeps the slot of the biome it replaces.
	local := s.LocalBiome(grass)
	if !reflect.DeepEqual(s.VisibleBiomes(), []string{rock, local}) {
		t.Fatal("local copy lost its place", s.VisibleBiomes())
	}
	// New biomes join at the end; deleted ones simply disappear.
	b := s.Biomes[local]
	b.Path, b.Name, b.Local = local+"_2", "Second", true
	s.Biomes[b.Path] = b
	if !reflect.DeepEqual(s.VisibleBiomes(), []string{rock, local, b.Path}) {
		t.Fatal("new biome misplaced", s.VisibleBiomes())
	}
	if !s.MoveBiome(b.Path, -2) || !reflect.DeepEqual(s.VisibleBiomes(), []string{b.Path, rock, local}) {
		t.Fatal("multi-step move failed", s.VisibleBiomes())
	}
	delete(s.Biomes, b.Path)
	if !reflect.DeepEqual(s.VisibleBiomes(), []string{rock, local}) {
		t.Fatal("removed biome still listed", s.VisibleBiomes())
	}
	if e = p.Save(); e != nil {
		t.Fatal(e)
	}
	c2 := load(t, c.Dme.RootFile)
	p2, e := Open(c2, definition(t, c2, "/datum/planet/test"))
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(p2.State.VisibleBiomes(), []string{rock, local}) || p2.Modified() {
		t.Fatal("arrangement did not survive a save", p2.State.VisibleBiomes(), p2.Modified())
	}
	if !reflect.DeepEqual(Clone(p2.State).BiomeOrder, p2.State.BiomeOrder) {
		t.Fatal("clone changed the arrangement")
	}
}
