package ruin

import (
	"fmt"
	"strings"

	"sdmm/internal/ship"
)

func (c *Catalog) Removal(t Template) (*ship.RemovalPlan, error) {
	if !strings.HasPrefix(t.Type, Type+"/") {
		return nil, fmt.Errorf("choose a ruin to remove")
	}
	variants := t.Variants
	if len(variants) == 0 {
		variants = []Template{t}
	}
	spec := ship.RemovalSpec{Name: t.Name}
	for _, variant := range variants {
		spec.Types = append(spec.Types, variant.Type)
	}
	for path, obj := range c.Dme.Objects {
		selected := false
		for _, root := range spec.Types {
			if path == root || strings.HasPrefix(path, root+"/") {
				selected = true
				break
			}
		}
		if !selected {
			continue
		}
		suffix := text(obj.Vars, "suffix")
		if suffix == "" {
			continue
		}
		file, err := ship.Inside(c.Dme.RootDir, strings.ReplaceAll(text(obj.Vars, "prefix")+suffix, "\\", "/"))
		if err != nil {
			return nil, err
		}
		spec.Maps = append(spec.Maps, file)
	}
	return ship.PlanRemoval(c.Dme, spec)
}
