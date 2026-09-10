// Package planet edits the biome tables consumed by Voidcrew's planet generator.
package planet

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmvars"
)

const PlanetType = "/datum/planet"
const GeneratorType = "/datum/map_generator/planet_generator"
const BiomeType = "/datum/biome"
const MegafaunaRoll = `"bluh bluh huge boss"`

var HeatKeys = []string{"coldest", "cold", "warm", "perfect", "hot", "hottest"}
var CaveKeys = []string{"coldest_cave", "cold_cave", "warm_cave", "hot_cave"}
var MoistureKeys = []string{"lowest_humidity", "low_humidity", "medium_humidity", "high_humidity", "highest_humidity"}
var HeatNames = []string{"Coldest", "Cold", "Warm", "Temperate", "Hot", "Hottest"}
var CaveNames = []string{"Coldest", "Cold", "Warm", "Hot"}
var MoistureNames = []string{"Driest", "Dry", "Balanced", "Wet", "Wettest"}

type Entry struct {
	Path   string
	Weight float64
}
type Table struct {
	Field, Name, ChanceField string
	Entries                  []Entry
	Chance                   float64
}
type Biome struct {
	Path, Parent, Name string
	Cave, Local        bool
	Created            bool
	Tables             []Table
}
type Generator struct {
	Zoom, Mountain, Closed   float64
	Iterations, Birth, Death int
	// Shares are each climate band's part of the map, in percent. Unset (all
	// zero) shares mean the generator's built-in cut points.
	Heat     [6]float64
	CaveHeat [4]float64
	Moisture [5]float64
}

// The generator's historical cut points, expressed as band shares.
var DefaultHeatShares = [6]float64{20, 20, 20, 5, 15, 20}
var DefaultCaveHeatShares = [4]float64{25, 25, 25, 25}
var DefaultMoistureShares = [5]float64{20, 20, 20, 20, 20}

func (g Generator) HeatShares() [6]float64 {
	if g.Heat == ([6]float64{}) {
		return DefaultHeatShares
	}
	return g.Heat
}
func (g Generator) CaveHeatShares() [4]float64 {
	if g.CaveHeat == ([4]float64{}) {
		return DefaultCaveHeatShares
	}
	return g.CaveHeat
}
func (g Generator) MoistureShares() [5]float64 {
	if g.Moisture == ([5]float64{}) {
		return DefaultMoistureShares
	}
	return g.Moisture
}

// BiasedShares reshapes base toward one end of its scale: bias 1 favours the
// last band, -1 the first, and 0 returns base itself. Both heat scales and the
// moisture scale use it, so one pad position means the same thing everywhere.
func BiasedShares(base []float64, bias float64) []float64 {
	out := biasedShares(base, bias)
	for i := range out {
		out[i] = math.Round(out[i]*10) / 10
	}
	return out
}

func biasedShares(base []float64, bias float64) []float64 {
	out := make([]float64, len(base))
	total := 0.0
	for i, share := range base {
		out[i] = share * math.Exp(3*bias*bandCentre(i, len(base)))
		total += out[i]
	}
	for i := range out {
		out[i] = out[i] / total * 100
	}
	return out
}

// bandCentre places band i on a -1..1 scale from the first band to the last.
func bandCentre(i, n int) float64 {
	if n < 2 {
		return 0
	}
	return float64(2*i)/float64(n-1) - 1
}

func shareCentre(shares []float64) float64 {
	total, sum := 0.0, 0.0
	for i, share := range shares {
		total += share
		sum += share * bandCentre(i, len(shares))
	}
	if total <= 0 {
		return 0
	}
	return sum / total
}

// ShareBias is the pad position whose BiasedShares sit at the same centre of
// mass as shares, so a handle reads back the bands however they were edited.
func ShareBias(shares, base []float64) float64 {
	target := shareCentre(shares)
	lo, hi := -1.0, 1.0
	if target <= shareCentre(biasedShares(base, lo)) {
		return lo
	}
	if target >= shareCentre(biasedShares(base, hi)) {
		return hi
	}
	for i := 0; i < 40; i++ {
		mid := (lo + hi) / 2
		if shareCentre(biasedShares(base, mid)) < target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}

// BalanceShares sets one band and scales the others so the total stays 100.
func BalanceShares(shares []float64, changed int, value float64) {
	value = math.Max(0, math.Min(100, value))
	others := 0.0
	for i, share := range shares {
		if i != changed {
			others += share
		}
	}
	rest := 100 - value
	for i := range shares {
		if i == changed {
			continue
		}
		if others > 0 {
			shares[i] = math.Round(shares[i]/others*rest*10) / 10
		} else {
			shares[i] = math.Round(rest/float64(len(shares)-1)*10) / 10
		}
	}
	shares[changed] = math.Round(value*10) / 10
}

type Seeds struct{ Height, Heat, Moisture, Detail uint32 }
type Definition struct {
	Schema, GameSize                     int
	EnvironmentPath, RuinPath            string
	Environment                          *Environment
	Ruins                                *RuinSettings
	RiverPath                            string
	Rivers                               *RiverSettings
	Path, Name, Overmap, Area, Generator string
	Surface, Caves                       [][]string
	Settings                             Generator
}
type Catalog struct {
	Dme     *dmenv.Dme
	Planets []Definition
	Biomes  map[string]Biome
	Errors  []string
	// ClimateShares is true when the project's planet generator declares the
	// band share lists, so the workshop may write them.
	ClimateShares bool
}
type State struct {
	Version                              int
	Definition                           Definition
	Biomes                               map[string]Biome
	Seeds                                Seeds
	Size                                 int
	New                                  bool
	BaseArea, BaseOvermap, BaseGenerator string
	// BiomeOrder is the mapper's arrangement of the biome list. It only shapes
	// the workshop's lists; generated DM keeps its stable sorted layout.
	BiomeOrder []string `json:",omitempty"`
}

var tableSpecs = []Table{
	{Field: "open_turf_types", Name: "Ground"},
	{Field: "closed_turf_types", Name: "Cave walls"},
	{Field: "flora_spawn_list", Name: "Plants", ChanceField: "flora_spawn_chance"},
	{Field: "feature_spawn_list", Name: "Features", ChanceField: "feature_spawn_chance"},
	{Field: "mob_spawn_list", Name: "Creatures", ChanceField: "mob_spawn_chance"},
	{Field: "dangerous_mob_spawn_list", Name: "Dangerous creatures"},
	{Field: "megafauna_spawn_list", Name: "Megafauna"},
}

func Label(path string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(path, BiomeType+"/"), PlanetType+"/")
	s = strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), "/", " / ")
	if s == "" {
		return "Untitled"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
func ID(name string) string {
	return strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(name), "_"), "_")
}
func Clone(s State) State {
	data, _ := json.Marshal(s)
	var out State
	_ = json.Unmarshal(data, &out)
	return out
}

// Split only at the outer level: DM's constant printer retains nested list().
func split(s string, separator byte) ([]string, error) {
	var out []string
	depth, start := 0, 0
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '(' {
			depth++
		}
		if c == ')' {
			depth--
		}
		if depth < 0 {
			return nil, fmt.Errorf("unbalanced list")
		}
		if depth == 0 && c == separator {
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if depth != 0 || quote != 0 {
		return nil, fmt.Errorf("unterminated list")
	}
	if tail := strings.TrimSpace(s[start:]); tail != "" {
		out = append(out, tail)
	}
	return out, nil
}
func list(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "null" || s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, "list(") || !strings.HasSuffix(s, ")") {
		return nil, fmt.Errorf("expected a literal list, got %s", s)
	}
	return split(s[5:len(s)-1], ',')
}
func weights(s string) ([]Entry, error) {
	items, err := list(s)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, item := range items {
		pair, err := split(item, '=')
		if err != nil || len(pair) < 1 || len(pair) > 2 {
			return nil, fmt.Errorf("invalid weighted item: %s", item)
		}
		e := Entry{Path: pair[0], Weight: 1}
		if len(pair) == 2 {
			e.Weight, err = strconv.ParseFloat(pair[1], 64)
			if err != nil {
				return nil, err
			}
		}
		if (!strings.HasPrefix(e.Path, "/") && e.Path != MegafaunaRoll) || !finite(e.Weight) || e.Weight <= 0 {
			return nil, fmt.Errorf("invalid weight: %s", item)
		}
		out = append(out, e)
	}
	// Associative DM lists can repeat a key; each occurrence reads its last
	// assigned weight. Present one choice with the same total probability.
	merged := []Entry{}
	counts, indices := map[string]int{}, map[string]int{}
	for _, e := range out {
		counts[e.Path]++
		if counts[e.Path] == 1 {
			indices[e.Path] = len(merged)
			merged = append(merged, e)
		}
		merged[indices[e.Path]].Weight = float64(counts[e.Path]) * e.Weight
	}
	return merged, nil
}
func climate(s string, keys []string) ([][]string, error) {
	items, err := list(s)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	rows := map[string]map[string]string{}
	for _, item := range items {
		pair, err := split(item, '=')
		if err != nil || len(pair) != 2 {
			return nil, fmt.Errorf("invalid climate row")
		}
		key, err := strconv.Unquote(pair[0])
		if err != nil {
			return nil, err
		}
		cols, err := list(pair[1])
		if err != nil {
			return nil, err
		}
		rows[key] = map[string]string{}
		for _, col := range cols {
			cell, err := split(col, '=')
			if err != nil || len(cell) != 2 {
				return nil, fmt.Errorf("invalid climate cell")
			}
			k, err := strconv.Unquote(cell[0])
			if err != nil {
				return nil, err
			}
			rows[key][k] = cell[1]
		}
	}
	result := make([][]string, len(keys))
	for i, k := range keys {
		result[i] = make([]string, 5)
		for j, h := range MoistureKeys {
			result[i][j] = rows[k][h]
			if result[i][j] == "" {
				return nil, fmt.Errorf("missing climate cell %s / %s", k, h)
			}
		}
	}
	return result, nil
}
func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

func Discover(dme *dmenv.Dme) (*Catalog, error) {
	if dme == nil || dme.Objects[GeneratorType] == nil {
		return nil, fmt.Errorf("This project has no Voidcrew planet generator.")
	}
	c := &Catalog{Dme: dme, Biomes: map[string]Biome{}}
	_, c.ClimateShares = dme.Objects[GeneratorType].Vars.Value("heat_shares")
	paths := make([]string, 0, len(dme.Objects))
	for path := range dme.Objects {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		o := dme.Objects[path]
		if !strings.HasPrefix(path, BiomeType+"/") {
			continue
		}
		b := Biome{Path: path, Parent: path, Name: o.Vars.TextV("workshop_name", Label(path)), Cave: strings.HasPrefix(path, BiomeType+"/cave")}
		valid := true
		for _, spec := range tableSpecs {
			if spec.Field == "closed_turf_types" && !b.Cave {
				continue
			}
			t := spec
			var err error
			t.Entries, err = weights(o.Vars.ValueV(t.Field, "null"))
			if err != nil {
				c.Errors = append(c.Errors, path+" / "+t.Name+": "+err.Error())
				valid = false
				break
			}
			t.Chance = float64(o.Vars.FloatV(t.ChanceField, 0))
			b.Tables = append(b.Tables, t)
		}
		if valid {
			c.Biomes[path] = b
		}
	}
	for _, path := range paths {
		o := dme.Objects[path]
		if !strings.HasPrefix(path, PlanetType+"/") {
			continue
		}
		d := Definition{Path: path, Name: Label(path), Generator: GeneratorType}
		var err error
		d.Surface, err = climate(o.Vars.ValueV("overworld_biomes", "null"), HeatKeys)
		if err == nil {
			d.Caves, err = climate(o.Vars.ValueV("cave_biomes", "null"), CaveKeys)
		}
		if err != nil {
			c.Errors = append(c.Errors, path+": "+err.Error())
			continue
		}
		for _, p := range paths {
			a := dme.Objects[p]
			if strings.HasPrefix(p, "/area/") && a.Vars.ValueV("planet_type", "") == path {
				if d.Area == "" {
					d.Area = p
					d.Generator = a.Vars.ValueV("map_generator", GeneratorType)
				}
			}
		}
		for _, p := range paths {
			a := dme.Objects[p]
			if strings.HasPrefix(p, "/datum/overmap/planet/") && a.Vars.ValueV("planet_template", "") == path {
				d.Overmap = p
				d.Name = a.Vars.TextV("name", d.Name)
				d.Area = a.Vars.ValueV("surface_area", d.Area)
				d.Generator = a.Vars.ValueV("mapgen", d.Generator)
				break
			}
		}
		if err := c.readSettings(&d); err != nil {
			c.Errors = append(c.Errors, path+": "+err.Error())
			continue
		}
		g := dme.Objects[d.Generator]
		if g == nil {
			c.Errors = append(c.Errors, path+": missing generator")
			continue
		}
		d.Settings = readGenerator(g.Vars)
		if c.Dme.Objects[RiverSettingsType] != nil {
			d.RiverPath = o.Vars.ValueV("river_settings", "null")
			d.Rivers, err = c.readRivers(d.RiverPath)
			if err != nil {
				c.Errors = append(c.Errors, path+": "+err.Error())
				continue
			}
		}
		c.Planets = append(c.Planets, d)
	}
	return c, nil
}
func readGenerator(v *dmvars.Variables) Generator {
	g := Generator{Zoom: float64(v.FloatV("perlin_zoom", 65)), Mountain: float64(v.FloatV("mountain_height", .85)), Closed: float64(v.FloatV("initial_closed_chance", 45)), Iterations: v.IntV("smoothing_iterations", 20), Birth: v.IntV("birth_limit", 4), Death: v.IntV("death_limit", 3)}
	g.Heat, g.CaveHeat, g.Moisture = DefaultHeatShares, DefaultCaveHeatShares, DefaultMoistureShares
	readShares(v, "heat_shares", g.Heat[:])
	readShares(v, "cave_heat_shares", g.CaveHeat[:])
	readShares(v, "humidity_shares", g.Moisture[:])
	return g
}

// readShares fills out from a numeric DM list; absent or malformed lists keep
// the defaults, matching how the game generator falls back.
func readShares(v *dmvars.Variables, name string, out []float64) {
	text, ok := v.Value(name)
	if !ok {
		return
	}
	items, err := list(text)
	if err != nil || len(items) != len(out) {
		return
	}
	parsed := make([]float64, len(out))
	for i, item := range items {
		f, err := strconv.ParseFloat(strings.TrimSpace(item), 64)
		if err != nil || !finite(f) || f < 0 {
			return
		}
		parsed[i] = f
	}
	copy(out, parsed)
}
func NewState(c *Catalog, d Definition) State {
	s := State{Version: 1, Definition: d, Biomes: map[string]Biome{}, Size: 123, Seeds: Seeds{Height: 12345, Heat: 23456, Moisture: 34567, Detail: 45678}, BaseArea: d.Area, BaseOvermap: d.Overmap, BaseGenerator: d.Generator}
	for _, grid := range [][][]string{d.Surface, d.Caves} {
		for _, row := range grid {
			for _, path := range row {
				if b, ok := c.Biomes[path]; ok {
					s.Biomes[path] = b
				}
			}
		}
	}
	return Clone(s)
}
func (s *State) UsedBiomes() []string {
	seen := map[string]bool{}
	var out []string
	for _, grid := range [][][]string{s.Definition.Surface, s.Definition.Caves} {
		for _, row := range grid {
			for _, p := range row {
				if !seen[p] {
					seen[p] = true
					out = append(out, p)
				}
			}
		}
	}
	return out
}
func (s *State) LocalBiome(path string) string {
	b, ok := s.Biomes[path]
	if !ok || b.Local {
		return path
	}
	base := BiomeType
	if b.Cave {
		base += "/cave"
	}
	base += "/workshop_" + ID(s.Definition.Path) + "_" + ID(strings.TrimPrefix(path, BiomeType+"/"))
	local := base
	for i := 2; ; i++ {
		if _, ok := s.Biomes[local]; !ok {
			break
		}
		local = fmt.Sprintf("%s_%d", base, i)
	}
	b.Path, b.Local, b.Parent = local, true, path
	s.remapRiverBiome(path, local)
	for i, p := range s.BiomeOrder {
		if p == path {
			s.BiomeOrder[i] = local
		}
	}
	b.Tables = append([]Table(nil), b.Tables...)
	for i := range b.Tables {
		b.Tables[i].Entries = append([]Entry(nil), b.Tables[i].Entries...)
	}
	s.Biomes[local] = b
	for _, grid := range [][][]string{s.Definition.Surface, s.Definition.Caves} {
		for _, row := range grid {
			for i, p := range row {
				if p == path {
					row[i] = local
				}
			}
		}
	}
	return local
}

// VisibleBiomes includes newly created, as-yet unassigned biomes in the palette.
func (s *State) VisibleBiomes() []string {
	paths := s.UsedBiomes()
	seen := map[string]bool{}
	for _, path := range paths {
		seen[path] = true
	}
	var extra []string
	for path, b := range s.Biomes {
		if b.Local && !seen[path] {
			extra = append(extra, path)
		}
	}
	sort.Strings(extra)
	paths = append(paths, extra...)
	if len(s.BiomeOrder) == 0 {
		return paths
	}
	// Arranged biomes come first; anything new keeps its default place after.
	present := map[string]bool{}
	for _, path := range paths {
		present[path] = true
	}
	placed := map[string]bool{}
	var out []string
	for _, path := range s.BiomeOrder {
		if present[path] && !placed[path] {
			placed[path] = true
			out = append(out, path)
		}
	}
	for _, path := range paths {
		if !placed[path] {
			out = append(out, path)
		}
	}
	return out
}

// MoveBiome shifts a listed biome by offset places and keeps the arrangement.
func (s *State) MoveBiome(path string, offset int) bool {
	paths := s.VisibleBiomes()
	at := -1
	for i, p := range paths {
		if p == path {
			at = i
		}
	}
	to := at + offset
	if at < 0 || to < 0 || to >= len(paths) || offset == 0 {
		return false
	}
	moved := paths[at]
	paths = append(paths[:at], paths[at+1:]...)
	paths = append(paths[:to], append([]string{moved}, paths[to:]...)...)
	s.BiomeOrder = paths
	return true
}
func (s *State) Validate(c *Catalog) error {
	if err := s.Definition.validateSettings(*s, c); err != nil {
		return err
	}
	if err := s.Definition.Rivers.validate(*s, c); err != nil {
		return err
	}
	g := s.Definition.Settings
	if !finite(g.Zoom) || g.Zoom < 1 || g.Zoom > 500 || !finite(g.Mountain) || g.Mountain < 0 || g.Mountain > 1 || !finite(g.Closed) || g.Closed < 0 || g.Closed > 100 || g.Iterations < 0 || g.Iterations > 50 || g.Birth < 0 || g.Birth > 8 || g.Death < 0 || g.Death > 8 {
		return fmt.Errorf("Terrain settings are outside the supported range.")
	}
	heat, cave, wet := g.HeatShares(), g.CaveHeatShares(), g.MoistureShares()
	for _, shares := range [][]float64{heat[:], cave[:], wet[:]} {
		total := 0.0
		for _, share := range shares {
			if !finite(share) || share < 0 || share > 100 {
				return fmt.Errorf("Climate shares must be between 0%% and 100%%.")
			}
			total += share
		}
		if total <= 0 {
			return fmt.Errorf("At least one climate band needs a share above zero.")
		}
	}
	if !c.ClimateShares && (heat != DefaultHeatShares || cave != DefaultCaveHeatShares || wet != DefaultMoistureShares) {
		return fmt.Errorf("This project's planet generator has no climate shares yet. Update the game code before changing the climate balance.")
	}
	if s.Size < 24 || s.Size > 256 {
		return fmt.Errorf("Preview size must be between 24 and 256 tiles.")
	}
	if len(s.Definition.Surface) == 0 && len(s.Definition.Caves) == 0 {
		return fmt.Errorf("Add a surface or a cave climate table.")
	}
	for n, grid := range [][][]string{s.Definition.Surface, s.Definition.Caves} {
		want := 6
		if n == 1 {
			want = 4
		}
		if len(grid) != 0 && len(grid) != want {
			return fmt.Errorf("Incomplete climate table.")
		}
		for _, row := range grid {
			if len(row) != 5 {
				return fmt.Errorf("Incomplete moisture row.")
			}
			for _, path := range row {
				b, ok := s.Biomes[path]
				if !ok {
					return fmt.Errorf("Biome %s could not be read.", path)
				}
				if n == 1 && !b.Cave {
					return fmt.Errorf("Caves need a biome with cave walls.")
				}
			}
		}
	}
	for _, path := range s.VisibleBiomes() {
		b := s.Biomes[path]
		for _, t := range b.Tables {
			if (t.Field == "open_turf_types" || t.Field == "closed_turf_types") && len(t.Entries) == 0 {
				return fmt.Errorf("%s needs at least one %s choice.", b.Name, t.Name)
			}
			if !finite(t.Chance) || t.Chance < 0 || t.Chance > 100 {
				return fmt.Errorf("%s chance must be between 0 and 100%%.", t.Name)
			}
			seen := map[string]bool{}
			for _, e := range t.Entries {
				if (c.Dme.Objects[e.Path] == nil && e.Path != MegafaunaRoll) || !Fits(t.Field, e.Path) {
					return fmt.Errorf("%s is not a valid %s choice.", e.Path, t.Name)
				}
				if !finite(e.Weight) || e.Weight <= 0 || e.Weight > 100000 || seen[e.Path] {
					return fmt.Errorf("%s has a duplicate or invalid weight.", t.Name)
				}
				seen[e.Path] = true
			}
		}
	}
	return nil
}
func Fits(field, path string) bool {
	switch field {
	case "open_turf_types", "river_turf":
		return strings.HasPrefix(path, "/turf/open/")
	case "closed_turf_types":
		return strings.HasPrefix(path, "/turf/closed/")
	case "mob_spawn_list", "dangerous_mob_spawn_list", "megafauna_spawn_list":
		return (field == "mob_spawn_list" && path == MegafaunaRoll) || strings.HasPrefix(path, "/mob/living/") || strings.HasPrefix(path, "/obj/structure/spawner/") || strings.HasPrefix(path, "/obj/effect/spawner/")
	default:
		return strings.HasPrefix(path, "/obj/")
	}
}
