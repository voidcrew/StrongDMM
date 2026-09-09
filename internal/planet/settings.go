package planet

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

const EnvironmentType = "/datum/planet_environment"
const RuinSettingsType = "/datum/planet_ruins"

type Environment struct {
	Area, Baseturf, Weather, WeatherTrait string
	Atmosphere                            *string // nil keeps the biome turf's air
	LightColor                            string
	LightAlpha, Gravity                   float64
}

type RuinSettings struct {
	Enabled          bool
	Theme            string
	Budget, Minerals float64
	Templates        []string // nil uses the theme pool; empty selects none
}

func typeValue(s string) string {
	if s == "null" {
		return ""
	}
	return s
}
func dmPath(s string) string {
	if s == "" {
		return "null"
	}
	return s
}
func dmBool(b bool) int {
	if b {
		return 1
	}
	return 0
}
func dmPaths(paths []string) string {
	if paths == nil {
		return "null"
	}
	return "list(" + strings.Join(paths, ", ") + ")"
}
func literalPaths(raw string) ([]string, error) {
	paths, err := list(raw)
	if err == nil && raw != "null" && paths == nil {
		paths = []string{}
	}
	return paths, err
}

func (c *Catalog) readSettings(d *Definition) error {
	v := c.Dme.Objects[d.Path].Vars
	d.Schema = v.IntV("definition_version", 0)
	if d.Schema == 0 {
		return nil
	}
	if d.Schema != 1 {
		return fmt.Errorf("Unsupported planet definition version %d", d.Schema)
	}
	d.GameSize = v.IntV("planet_size", 123)
	d.Generator = v.ValueV("terrain_generator", GeneratorType)
	d.EnvironmentPath = typeValue(v.ValueV("environment", "null"))
	if o := c.Dme.Objects[d.EnvironmentPath]; d.EnvironmentPath != "" && o != nil {
		e := o.Vars
		d.Environment = &Environment{Area: typeValue(e.ValueV("area_type", "null")), Baseturf: typeValue(e.ValueV("baseturf", "null")), Weather: typeValue(e.ValueV("weather_type", "null")), WeatherTrait: e.TextV("weather_trait", ""), LightColor: e.TextV("light_color", "#FFFFFF"), LightAlpha: float64(e.FloatV("light_alpha", 255)), Gravity: float64(e.FloatV("gravity", 1))}
		if e.ValueV("atmosphere", "null") != "null" {
			air := e.TextV("atmosphere", "")
			d.Environment.Atmosphere = &air
		}
		d.Area = d.Environment.Area
	} else {
		return fmt.Errorf("Missing planet environment")
	}
	d.RuinPath = typeValue(v.ValueV("ruin_settings", "null"))
	if o := c.Dme.Objects[d.RuinPath]; d.RuinPath != "" && o != nil {
		r := o.Vars
		paths, err := literalPaths(r.ValueV("templates", "null"))
		if err != nil {
			return err
		}
		d.Ruins = &RuinSettings{Enabled: r.IntV("enabled", 1) != 0, Theme: r.TextV("theme", ""), Budget: float64(r.FloatV("budget_multiplier", 1)), Minerals: float64(r.FloatV("mineral_budget", 15)), Templates: paths}
	} else {
		d.Ruins = &RuinSettings{Budget: 1, Minerals: 15}
	}
	return nil
}

func (d Definition) validateSettings(s State, c *Catalog) error {
	if d.Schema == 0 {
		return nil
	}
	if d.Schema != 1 || d.GameSize < 24 || d.GameSize > 123 {
		return fmt.Errorf("Planet size must be between 24 and 123 tiles.")
	}
	e := d.Environment
	if e == nil {
		return fmt.Errorf("Choose a planet environment.")
	}
	if !strings.HasPrefix(e.Area, "/area/overmap_encounter/planetoid/") || (c.Dme.Objects[e.Area] == nil && !(s.New && e.Area == d.Area)) {
		return fmt.Errorf("Unknown planet surface area.")
	}
	if !strings.HasPrefix(e.Baseturf, "/turf/open/") || c.Dme.Objects[e.Baseturf] == nil {
		return fmt.Errorf("Choose the ground beneath this planet.")
	}
	if e.Weather != "" && (!strings.HasPrefix(e.Weather, "/datum/weather/") || c.Dme.Objects[e.Weather] == nil || e.WeatherTrait == "") {
		return fmt.Errorf("Choose a supported weather pattern.")
	}
	if !regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(e.LightColor) || !finite(e.LightAlpha) || e.LightAlpha < 0 || e.LightAlpha > 255 || !finite(e.Gravity) || e.Gravity < 0 || e.Gravity > 5 {
		return fmt.Errorf("Environment lighting or gravity is outside the supported range.")
	}
	if e.Atmosphere != nil && (len(*e.Atmosphere) > 500 || strings.ContainsAny(*e.Atmosphere, "\r\n")) {
		return fmt.Errorf("Invalid atmosphere.")
	}
	r := d.Ruins
	if r == nil || !finite(r.Budget) || r.Budget < 0 || r.Budget > 5 || !finite(r.Minerals) || r.Minerals < 0 || r.Minerals > 100 {
		return fmt.Errorf("Ruin settings are outside the supported range.")
	}
	for _, path := range r.Templates {
		o := c.Dme.Objects[path]
		if o == nil || !strings.HasPrefix(path, "/datum/map_template/ruin/") || o.Vars.TextV("id", "") == "" || o.Vars.TextV("ruin_type", "") == "Space Ruins" {
			return fmt.Errorf("Unknown planet ruin: %s", path)
		}
	}
	return nil
}

func (p *Project) environmentPath() string { return EnvironmentType + "/workshop_" + p.identifier() }
func (p *Project) ruinPath() string        { return RuinSettingsType + "/workshop_" + p.identifier() }
func (p *Project) managesEnvironment() bool {
	d := p.State.Definition
	return d.Environment != nil && (d.EnvironmentPath == p.environmentPath() || p.UnsavedNew() || !reflect.DeepEqual(d.Environment, p.saved.Definition.Environment))
}
func (p *Project) managesRuins() bool {
	d := p.State.Definition
	return d.Ruins != nil && (d.RuinPath == p.ruinPath() || p.UnsavedNew() || !reflect.DeepEqual(d.Ruins, p.saved.Definition.Ruins))
}
func (p *Project) renderEnvironment() string {
	e := p.State.Definition.Environment
	air := "null"
	if e.Atmosphere != nil {
		air = quote(*e.Atmosphere)
	}
	trait := "null"
	if e.WeatherTrait != "" {
		trait = quote(e.WeatherTrait)
	}
	return fmt.Sprintf("%s\n\tarea_type = %s\n\tbaseturf = %s\n\tatmosphere = %s\n\tweather_type = %s\n\tweather_trait = %s\n\tlight_color = %s\n\tlight_alpha = %s\n\tgravity = %s\n\n", p.environmentPath(), e.Area, e.Baseturf, air, dmPath(e.Weather), trait, quote(e.LightColor), number(e.LightAlpha), number(e.Gravity))
}
func (p *Project) renderRuins() string {
	r := p.State.Definition.Ruins
	theme := "null"
	if r.Theme != "" {
		theme = quote(r.Theme)
	}
	return fmt.Sprintf("%s\n\tenabled = %d\n\ttheme = %s\n\tbudget_multiplier = %s\n\tmineral_budget = %s\n\ttemplates = %s\n\n", p.ruinPath(), dmBool(r.Enabled), theme, number(r.Budget), number(r.Minerals), dmPaths(r.Templates))
}

// AcceptsRuin uses the live planet draft, including an explicit template selection.
func (c *Catalog) AcceptsRuin(s State, path string) bool {
	o := c.Dme.Objects[path]
	if o == nil {
		return false
	}
	if r := s.Definition.Ruins; r != nil {
		if !r.Enabled {
			return false
		}
		if r.Templates != nil {
			for _, p := range r.Templates {
				if p == path {
					return true
				}
			}
			return false
		}
		return r.Theme != "" && r.Theme == o.Vars.TextV("ruin_type", "")
	}
	registration := c.Dme.Objects[s.Definition.Overmap]
	if registration == nil {
		registration = c.Dme.Objects[s.BaseOvermap]
	}
	return registration != nil && registration.Vars.ValueV("ruin_type", "null") != "null" && registration.Vars.ValueV("ruin_type", "null") == o.Vars.ValueV("ruin_type", "null")
}
