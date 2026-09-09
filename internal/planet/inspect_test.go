package planet

import "testing"

func TestPreviewRetainsSpawnTable(t *testing.T) {
	c := fixture(t)
	for _, field := range []string{"flora_spawn_list", "feature_spawn_list", "mob_spawn_list"} {
		s := NewState(c, definition(t, c, "/datum/planet/test"))
		b := s.Biomes["/datum/biome/grass"]
		path := "/obj/tree"
		if field == "mob_spawn_list" {
			path = "/mob/living/test"
		}
		for i := range b.Tables {
			if b.Tables[i].ChanceField != "" {
				b.Tables[i].Chance = 0
			}
			if b.Tables[i].Field == "flora_spawn_list" || b.Tables[i].Field == "feature_spawn_list" || b.Tables[i].Field == "mob_spawn_list" {
				// Both object groups deliberately contain the same choice.
				choice := "/obj/tree"
				if b.Tables[i].Field == "mob_spawn_list" {
					choice = "/mob/living/test"
				}
				b.Tables[i].Entries = []Entry{{Path: choice, Weight: 1}}
			}
			if b.Tables[i].Field == field {
				b.Tables[i].Chance = 100
			}
		}
		s.Biomes[b.Path] = b
		p, err := Generate(c, s, c.Dme, PreviewOptions{Biome: b.Path, Populate: true})
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for i, cell := range p.Cells {
			if cell.Spawn == "" {
				continue
			}
			count++
			if cell.Spawn != path || cell.SpawnField != field {
				t.Fatal("wrong spawn provenance", cell)
			}
			instances := p.Map.Tiles[i].Instances()
			if instances[len(instances)-1].Prefab().Path() != cell.Spawn {
				t.Fatal("spawn metadata does not match the displayed map")
			}
		}
		if count == 0 {
			t.Fatal("no population to inspect", field)
		}
		p, err = Generate(c, s, c.Dme, PreviewOptions{Biome: b.Path})
		if err != nil {
			t.Fatal(err)
		}
		for _, cell := range p.Cells {
			if cell.Spawn != "" || cell.SpawnField != "" {
				t.Fatal("hidden population remained selectable")
			}
		}
	}
}
