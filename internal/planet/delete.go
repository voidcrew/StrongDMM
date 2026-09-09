package planet

import (
	"fmt"
	"sort"
)

// BiomeUse counts assignments, including climates outside the preview's seed.
func (p *Project) BiomeUse(path string) (surface, caves int) {
	for layer, grid := range [][][]string{p.State.Definition.Surface, p.State.Definition.Caves} {
		for _, row := range grid {
			for _, cell := range row {
				if cell == path {
					if layer == 0 {
						surface++
					} else {
						caves++
					}
				}
			}
		}
	}
	return
}

// BiomeChoices merges edited biomes with the project library. A deleted local
// definition must not be offered from the old catalog before the next save.
func (p *Project) BiomeChoices(caves bool) []Biome {
	all := map[string]Biome{}
	for path, b := range p.Catalog.Biomes {
		if old := p.saved.Biomes[path]; old.Local {
			if _, exists := p.State.Biomes[path]; !exists {
				continue
			}
		}
		all[path] = b
	}
	for path, b := range p.State.Biomes {
		all[path] = b
	}
	var out []Biome
	for _, b := range all {
		if !caves || b.Cave {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name+out[i].Path < out[j].Name+out[j].Path })
	return out
}

func (p *Project) CheckDeleteBiome(path, replacement string) error {
	b, ok := p.State.Biomes[path]
	if !ok {
		return fmt.Errorf("Choose a biome to delete.")
	}
	surface, caves := p.BiomeUse(path)
	if surface+caves > 0 {
		valid := false
		for _, choice := range p.BiomeChoices(caves > 0) {
			if choice.Path == replacement && replacement != path {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("Choose a replacement for the climate cells that use this biome.")
		}
	}
	if b.Local {
		// These declarations belong to this planet's generated file. Do not
		// remove one if another planet or a handwritten subtype still needs it.
		for _, d := range p.Catalog.Planets {
			if d.Path == p.State.Definition.Path {
				continue
			}
			s := State{Definition: d}
			uses := s.UsedBiomes()
			if d.Rivers != nil {
				uses = append(uses, d.Rivers.Biomes...)
			}
			for _, used := range uses {
				if used == path {
					return fmt.Errorf("%s also uses this custom biome. Replace it there before deleting it here.", d.Name)
				}
			}
		}
		for childPath, child := range p.Catalog.Dme.Objects {
			if parent := child.Parent(); parent != nil && parent.Path == path {
				if _, exists := p.State.Biomes[childPath]; !exists && p.saved.Biomes[childPath].Local {
					continue // This owned child has already been deleted.
				}
				if own := p.State.Biomes[childPath]; !own.Local {
					return fmt.Errorf("%s inherits this custom biome. Update its starting biome before deleting this definition.", Label(childPath))
				}
			}
		}
	}
	return nil
}

// DeleteBiome replaces every climate reference atomically. Shared source
// biomes are removed only from this planet; owned definitions are removed too.
func (p *Project) DeleteBiome(path, replacement string) error {
	if err := p.CheckDeleteBiome(path, replacement); err != nil {
		return err
	}
	next := Clone(p.State)
	b := next.Biomes[path]
	surface, caves := p.BiomeUse(path)
	if surface+caves > 0 {
		for _, choice := range p.BiomeChoices(caves > 0) {
			if choice.Path == replacement {
				next.Biomes[replacement] = choice
				break
			}
		}
		for _, grid := range [][][]string{next.Definition.Surface, next.Definition.Caves} {
			for _, row := range grid {
				for i, cell := range row {
					if cell == path {
						row[i] = replacement
					}
				}
			}
		}
	}
	if b.Local {
		// Owned copies already contain every editable table and chance. Keep
		// their original behavior while bypassing the removed generated parent.
		for childPath, child := range next.Biomes {
			if child.Local && child.Parent == path {
				child.Parent = b.Parent
				next.Biomes[childPath] = child
			}
		}
	}
	delete(next.Biomes, path)
	next.remapRiverBiome(path, replacement)
	// Catalog entries may share table slices; isolate any imported replacement.
	p.State = Clone(next)
	return nil
}
