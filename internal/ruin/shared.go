package ruin

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"sdmm/internal/ship"
)

func (p *Project) registrationValues(props Properties) map[string]string {
	values := props.values()
	if p.Template.Group != "" {
		values["name"] = quote(props.Name + " (" + p.groupLocation + ")")
	}
	return values
}

func (p *Project) readProperties() (Properties, error) {
	props, err := ReadProperties(p.Catalog.Dme.Objects[p.Template.Type])
	if p.Template.Group != "" {
		props.Name = strings.TrimSuffix(props.Name, " ("+p.groupLocation+")")
	}
	return props, err
}

func applyPropertyChanges(target, before, after Properties) Properties {
	if before.Name != after.Name {
		target.Name = after.Name
	}
	if before.Description != after.Description {
		target.Description = after.Description
	}
	if before.Cost != after.Cost {
		target.Cost = after.Cost
	}
	if before.MineralCost != after.MineralCost {
		target.MineralCost = after.MineralCost
	}
	if before.Weight != after.Weight {
		target.Weight = after.Weight
	}
	if before.AllowDuplicates != after.AllowDuplicates {
		target.AllowDuplicates = after.AllowDuplicates
	}
	if before.AlwaysPlace != after.AlwaysPlace {
		target.AlwaysPlace = after.AlwaysPlace
	}
	if before.Unpickable != after.Unpickable {
		target.Unpickable = after.Unpickable
	}
	return target
}

// One save transaction registers the same map for all of its destinations.
// Inherited biome settings (such as ice ceilings) stay on each native parent.
func (p *Project) Changes() ([]ship.FileChange, error) {
	if !p.Modified() {
		return nil, nil
	}
	if p.Template.Group != "" {
		if err := p.Catalog.NameError(p.Properties.Name, p.Template.Type); err != nil {
			return nil, err
		}
	}
	files := map[string]ship.FileChange{}
	dme := p.dme
	dme.After = dme.Before
	for _, member := range append([]*Project{p}, p.companions...) {
		if member != p {
			member.Properties = applyPropertyChanges(member.initial, p.initial, p.Properties)
		}
		if !bytes.Equal(member.dme.Before, dme.Before) {
			return nil, fmt.Errorf("the project includes changed while preparing the ruin")
		}
		changes, err := member.changesOne()
		if err != nil {
			return nil, err
		}
		if len(changes) == 0 {
			continue
		}
		for _, c := range changes {
			if c.Path == dme.Path {
				continue
			}
			if prior, ok := files[c.Path]; ok {
				if prior.Existed != c.Existed || !bytes.Equal(prior.Before, c.Before) {
					return nil, fmt.Errorf("conflicting ruin files: %s", c.Path)
				}
				if bytes.Equal(c.Before, c.After) {
					continue
				}
				if !bytes.Equal(prior.Before, prior.After) && !bytes.Equal(prior.After, c.After) {
					return nil, fmt.Errorf("conflicting ruin edits: %s", c.Path)
				}
			}
			files[c.Path] = c
		}
		rel, err := filepath.Rel(p.Catalog.Dme.RootDir, member.source.Path)
		if err != nil {
			return nil, err
		}
		dme.After, err = include(dme.After, filepath.ToSlash(rel))
		if err != nil {
			return nil, err
		}
	}
	files[dme.Path] = dme
	var result []ship.FileChange
	for _, c := range files {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}
