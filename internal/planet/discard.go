package planet

import (
	"fmt"
	"reflect"
)

func (p *Project) savedBiome(path string) (Biome, bool) {
	baseline := p.saved
	if p.UnsavedNew() {
		baseline = p.starting
	}
	if b, ok := baseline.Biomes[path]; ok {
		return b, true
	}
	if b, ok := p.starting.Biomes[path]; ok {
		return b, true
	}
	b := p.State.Biomes[path]
	if b.Local && !b.Created {
		original, ok := baseline.Biomes[b.Parent]
		return original, ok
	}
	return Biome{}, false
}

// RememberNewBiome gives an unsaved biome a starting point for Discard changes.
func (p *Project) RememberNewBiome(path string) {
	if p.starting.Biomes == nil {
		p.starting.Biomes = map[string]Biome{}
	}
	b := p.State.Biomes[path]
	copy := Clone(State{Biomes: map[string]Biome{path: b}})
	p.starting.Biomes[path] = copy.Biomes[path]
}

func (p *Project) CanDiscardBiome(path string) bool {
	original, ok := p.savedBiome(path)
	return ok && !reflect.DeepEqual(original, p.State.Biomes[path])
}

func (p *Project) DiscardBiomeChanges(path string) (string, error) {
	original, ok := p.savedBiome(path)
	if !ok {
		return path, fmt.Errorf("This biome has no saved version. Delete biome removes a new biome.")
	}
	next := Clone(p.State)
	if path != original.Path {
		next.remapRiverBiome(path, original.Path)
		// Discard the automatic local copy made when editing a shared biome.
		for _, grid := range [][][]string{next.Definition.Surface, next.Definition.Caves} {
			for _, row := range grid {
				for i, cell := range row {
					if cell == path {
						row[i] = original.Path
					}
				}
			}
		}
		for childPath, child := range next.Biomes {
			if child.Local && child.Parent == path {
				child.Parent = original.Path
				next.Biomes[childPath] = child
			}
		}
		delete(next.Biomes, path)
	}
	next.Biomes[original.Path] = original
	p.State = Clone(next)
	return original.Path, nil
}
