package planet

import (
	"fmt"
	"reflect"
	"strings"
)

const RiverSettingsType = "/datum/planet_rivers"

type RiverSettings struct {
	Enabled              bool
	Turf                 string
	Nodes                int
	Spread, Loss, Detour float64
	Biomes               []string // nil: all; empty: none
}

func defaultRivers() *RiverSettings {
	return &RiverSettings{Nodes: 4, Spread: 25, Loss: 11, Detour: 20}
}

func (c *Catalog) readRivers(path string) (*RiverSettings, error) {
	r := defaultRivers()
	if path == "null" || path == "" {
		return r, nil
	}
	o := c.Dme.Objects[path]
	if o == nil || (path != RiverSettingsType && !strings.HasPrefix(path, RiverSettingsType+"/")) {
		return nil, fmt.Errorf("Unknown river settings: %s", path)
	}
	v := o.Vars
	r.Enabled, r.Turf = v.IntV("enabled", 0) != 0, v.ValueV("turf_type", "null")
	if r.Turf == "null" {
		r.Turf = ""
	}
	r.Nodes = v.IntV("node_count", 4)
	r.Spread, r.Loss, r.Detour = float64(v.FloatV("spread_chance", 25)), float64(v.FloatV("spread_loss", 11)), float64(v.FloatV("detour_chance", 20))
	raw := v.ValueV("biomes", "null")
	var err error
	r.Biomes, err = list(raw)
	if raw != "null" && err == nil && r.Biomes == nil {
		r.Biomes = []string{}
	}
	return r, err
}

func (r *RiverSettings) validate(s State, c *Catalog) error {
	if r == nil {
		return nil
	}
	if c.Dme.Objects[RiverSettingsType] == nil {
		return fmt.Errorf("This game project does not support editable rivers yet.")
	}
	if (r.Enabled || r.Turf != "") && (!strings.HasPrefix(r.Turf, "/turf/open/") || c.Dme.Objects[r.Turf] == nil) {
		return fmt.Errorf("Choose an open turf for the rivers.")
	}
	if r.Nodes < 2 || r.Nodes > 12 || !finite(r.Spread) || r.Spread < 0 || r.Spread > 40 || !finite(r.Loss) || r.Loss < 10 || r.Loss > 100 || !finite(r.Detour) || r.Detour < 0 || r.Detour > 60 {
		return fmt.Errorf("River settings are outside the supported range.")
	}
	for _, path := range r.Biomes {
		if _, local := s.Biomes[path]; !local {
			if _, exists := c.Biomes[path]; !exists {
				return fmt.Errorf("Unknown river biome: %s", path)
			}
		}
	}
	return nil
}

func (s *State) remapRiverBiome(old, next string) {
	r := s.Definition.Rivers
	if r == nil || r.Biomes == nil {
		return
	}
	paths, seen := []string{}, map[string]bool{}
	for _, path := range r.Biomes {
		if path == old {
			path = next
		}
		if path != "" && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	r.Biomes = paths
}

func (p *Project) riverPath() string { return RiverSettingsType + "/workshop_" + p.identifier() }

func (p *Project) managesRivers() bool {
	d := p.State.Definition
	return d.Rivers != nil && (d.RiverPath == p.riverPath() || p.UnsavedNew() || !reflect.DeepEqual(d.Rivers, p.saved.Definition.Rivers))
}

func (p *Project) renderRivers() string {
	r := p.State.Definition.Rivers
	turf, biomes, enabled := r.Turf, "null", 0
	if turf == "" {
		turf = "null"
	}
	if r.Enabled {
		enabled = 1
	}
	if r.Biomes != nil {
		biomes = "list(" + strings.Join(r.Biomes, ", ") + ")"
	}
	return fmt.Sprintf("%s\n\tenabled = %d\n\tturf_type = %s\n\tnode_count = %d\n\tspread_chance = %s\n\tspread_loss = %s\n\tdetour_chance = %s\n\tbiomes = %s\n\n", p.riverPath(), enabled, turf, r.Nodes, number(r.Spread), number(r.Loss), number(r.Detour), biomes)
}
