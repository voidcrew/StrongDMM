package planet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sdmm/internal/ship"
)

type Project struct {
	Catalog    *Catalog
	State      State
	saved      State
	starting   State
	sources    map[string]ship.FileChange
	meta, code string
}

const markerRegistry = "voidcrew/modules/overmap/code/controllers/subsystem/overmap.dm"

func Open(c *Catalog, d Definition) (*Project, error) {
	p := &Project{Catalog: c, State: NewState(c, d), sources: map[string]ship.FileChange{}}
	p.paths()
	if err := p.capture(); err != nil {
		return nil, err
	}
	if data := p.sources[p.meta].Before; len(data) > 0 {
		var saved State
		if err := json.Unmarshal(data, &saved); err != nil || saved.Version != 1 || saved.Definition.Path != d.Path {
			return nil, fmt.Errorf("Invalid planet workshop project: %s", filepath.Base(p.meta))
		}
		// DM remains authoritative after a mapper or coder edits it outside the workshop.
		p.State.Seeds, p.State.Size = saved.Seeds, saved.Size
		p.State.New, p.State.BaseArea, p.State.BaseOvermap, p.State.BaseGenerator = saved.New, saved.BaseArea, saved.BaseOvermap, saved.BaseGenerator
		if err := p.capture(); err != nil {
			return nil, err
		}
		for path, old := range saved.Biomes {
			if old.Local {
				if b, ok := c.Biomes[path]; ok {
					b.Local, b.Parent, b.Created = true, old.Parent, old.Created
					p.State.Biomes[path] = b
				}
			}
		}
		p.saved = Clone(p.State)
		if !bytes.Equal(p.generated(), p.sources[p.code].Before) {
			return nil, fmt.Errorf("Planet workshop definitions changed outside the editor. Keep those changes and reconcile %s before editing here.", filepath.Base(p.code))
		}
	}
	p.saved = Clone(p.State)
	return p, nil
}
func Create(c *Catalog, base Definition, name string, blank bool) (*Project, error) {
	if base.Overmap == "" || base.Area == "" || c.Dme.Objects[base.Overmap] == nil || c.Dme.Objects[base.Area] == nil {
		return nil, fmt.Errorf("Choose a starting planet with a surface area and overmap registration.")
	}
	name = strings.TrimSpace(name)
	id := ID(name)
	if id == "" || len(name) > 80 {
		return nil, fmt.Errorf("Enter a planet name (1-80 characters).")
	}
	for _, d := range c.Planets {
		if strings.EqualFold(strings.Join(strings.Fields(d.Name), " "), strings.Join(strings.Fields(name), " ")) {
			return nil, fmt.Errorf("A planet already uses that name.")
		}
	}
	p := &Project{Catalog: c, State: NewState(c, base), sources: map[string]ship.FileChange{}}
	s := &p.State
	s.New = true
	s.Definition.Path = PlanetType + "/workshop_" + id
	s.Definition.Name = name
	s.Definition.Area = "/area/overmap_encounter/planetoid/workshop_" + id
	s.Definition.Overmap = "/datum/overmap/planet/workshop_" + id
	s.Definition.Generator = GeneratorType + "/workshop_" + id
	if s.Definition.Environment != nil {
		s.Definition.Environment.Area = s.Definition.Area
	}
	for _, path := range []string{s.Definition.Path, s.Definition.Area, s.Definition.Overmap, s.Definition.Generator} {
		if c.Dme.Objects[path] != nil {
			return nil, fmt.Errorf("That name's identifier is already in use.")
		}
	}
	if blank {
		if s.Definition.Rivers != nil {
			s.Definition.Rivers.Enabled = false
		}
		if s.Definition.Ruins != nil {
			s.Definition.Ruins.Enabled = false
		}
		if len(s.Definition.Surface) == 0 {
			return nil, fmt.Errorf("Choose a starting planet with surface ground.")
		}
		ground := s.Definition.Surface[0][0]
		b := s.Biomes[ground]
		for i := range b.Tables {
			if b.Tables[i].Field != "open_turf_types" {
				b.Tables[i].Entries = nil
				b.Tables[i].Chance = 0
			} else if len(b.Tables[i].Entries) > 1 {
				b.Tables[i].Entries = b.Tables[i].Entries[:1]
			}
		}
		s.Biomes[ground] = b
		for _, row := range s.Definition.Surface {
			for j := range row {
				row[j] = ground
			}
		}
		s.Definition.Caves = nil
		local := s.LocalBiome(ground)
		b = s.Biomes[local]
		b.Name = "Open ground"
		s.Biomes[local] = b
	}
	p.paths()
	if err := p.capture(); err != nil {
		return nil, err
	}
	if p.sources[p.meta].Existed || p.sources[p.code].Existed {
		return nil, fmt.Errorf("A saved planet already uses that name.")
	}
	p.starting = Clone(p.State)
	return p, nil
}
func (p *Project) paths() {
	id := ID(strings.TrimPrefix(p.State.Definition.Path, PlanetType+"/"))
	p.meta = filepath.Join(p.Catalog.Dme.RootDir, "voidcrew", "mapping", "planet_projects", id+".json")
	p.code = filepath.Join(p.Catalog.Dme.RootDir, "voidcrew", "mapping", "planet_projects", id+".dm")
}
func (p *Project) capture() error {
	paths := []string{p.meta, p.code, p.Catalog.Dme.RootFile}
	if p.State.New {
		paths = append(paths, filepath.Join(p.Catalog.Dme.RootDir, markerRegistry))
	}
	for _, path := range []string{p.State.Definition.Path, p.State.Definition.Overmap, p.State.Definition.RiverPath, p.State.Definition.EnvironmentPath, p.State.Definition.RuinPath} {
		if o := p.Catalog.Dme.Objects[path]; o != nil && o.Location.File != "" {
			paths = append(paths, filepath.Join(p.Catalog.Dme.RootDir, o.Location.File))
		}
	}
	for path, o := range p.Catalog.Dme.Objects {
		if strings.HasPrefix(path, "/area/") && o.Vars.ValueV("planet_type", "") == p.State.Definition.Path && o.Location.File != "" {
			paths = append(paths, filepath.Join(p.Catalog.Dme.RootDir, o.Location.File))
		}
	}
	for _, path := range paths {
		if _, ok := p.sources[path]; ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		p.sources[path] = ship.FileChange{Path: path, Before: data, Existed: err == nil}
	}
	return nil
}
func (p *Project) Modified() bool        { return !reflect.DeepEqual(p.State, p.saved) }
func (p *Project) GeneratedPath() string { return p.code }
func (p *Project) UnsavedNew() bool      { return p.State.New && !p.sources[p.meta].Existed }

// ObserveSave advances shared source snapshots after a known successful save in
// this workshop. Unrecognized/external edits still fail the normal save checks.
func (p *Project) ObserveSave(changes []ship.FileChange, catalog *Catalog) {
	for _, change := range changes {
		source, ok := p.sources[change.Path]
		if !ok || change.Path == p.meta || change.Path == p.code {
			continue
		}
		if source.Existed == change.Existed && bytes.Equal(source.Before, change.Before) {
			source.Before, source.Existed = append([]byte(nil), change.After...), true
			p.sources[change.Path] = source
		}
	}
	p.Catalog = catalog
}
func (p *Project) generated() []byte {
	var b strings.Builder
	s := p.State
	d := s.Definition
	b.WriteString("// Planet Workshop definitions.\n\n")
	paths := make([]string, 0)
	for path, v := range s.Biomes {
		if v.Local {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		v := s.Biomes[path]
		nameField := "var/workshop_name"
		if parent := p.Catalog.Dme.Objects[v.Parent]; parent != nil {
			if _, exists := parent.Vars.Value("workshop_name"); exists {
				nameField = "workshop_name"
			}
		}
		fmt.Fprintf(&b, "%s\n\tparent_type = %s\n\t%s = %s\n", v.Path, v.Parent, nameField, quote(v.Name))
		for _, t := range v.Tables {
			fmt.Fprintf(&b, "\t%s = %s\n", t.Field, renderWeights(t.Entries))
			if t.ChanceField != "" {
				fmt.Fprintf(&b, "\t%s = %s\n", t.ChanceField, number(t.Chance))
			}
		}
		b.WriteString("\n")
	}
	customGenerator := s.New || d.Generator == p.generatorPath() || d.Settings != p.saved.Definition.Settings
	if p.managesRivers() {
		b.WriteString(p.renderRivers())
	}
	if p.managesEnvironment() {
		b.WriteString(p.renderEnvironment())
	}
	if p.managesRuins() {
		b.WriteString(p.renderRuins())
	}
	if customGenerator {
		fmt.Fprintf(&b, "%s\n\tparent_type = %s\n", p.generatorPath(), s.BaseGenerator)
		g := d.Settings
		fmt.Fprintf(&b, "\tperlin_zoom = %s\n\tmountain_height = %s\n\tinitial_closed_chance = %s\n\tsmoothing_iterations = %d\n\tbirth_limit = %d\n\tdeath_limit = %d\n\n", number(g.Zoom), number(g.Mountain), number(g.Closed), g.Iterations, g.Birth, g.Death)
	}
	if s.New {
		fmt.Fprintf(&b, "%s\n\toverworld_biomes = %s\n\tcave_biomes = %s\n", d.Path, renderClimate(d.Surface, HeatKeys), renderClimate(d.Caves, CaveKeys))
		if d.Schema > 0 {
			fmt.Fprintf(&b, "\tterrain_generator = %s\n\tplanet_size = %d\n", p.generatorPath(), d.GameSize)
		}
		if p.managesEnvironment() {
			fmt.Fprintf(&b, "\tenvironment = %s\n", p.environmentPath())
		}
		if p.managesRuins() {
			fmt.Fprintf(&b, "\truin_settings = %s\n", p.ruinPath())
		}
		if p.managesRivers() {
			fmt.Fprintf(&b, "\triver_settings = %s\n", p.riverPath())
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s\n\tparent_type = %s\n\tname = %s\n\tplanet_type = %s\n\tmap_generator = %s\n\n", d.Area, s.BaseArea, quote(d.Name), d.Path, p.generatorPath())
		fmt.Fprintf(&b, "%s\n\tparent_type = %s\n\tname = %s\n\tplanet_template = %s\n\tmapgen = %s\n\ttarget_area = %s\n\tsurface_area = %s\n\n", d.Overmap, s.BaseOvermap, quote(d.Name), d.Path, p.generatorPath(), d.Area, d.Area)
		fmt.Fprintf(&b, "%s\n\tplanet = %s\n", p.markerPath(), d.Overmap)
	}
	return []byte(b.String())
}
func (p *Project) generatorPath() string {
	return GeneratorType + "/workshop_" + p.identifier()
}
func (p *Project) identifier() string {
	return ID(strings.TrimPrefix(strings.TrimPrefix(p.State.Definition.Path, PlanetType+"/"), "workshop_"))
}
func (p *Project) markerPath() string {
	return "/obj/structure/overmap/planet/workshop_" + p.identifier()
}
func number(n float64) string { return strconv.FormatFloat(n, 'f', -1, 64) }
func quote(s string) string {
	q := strconv.Quote(s)
	return strings.ReplaceAll(strings.ReplaceAll(q, "[", `\[`), "]", `\]`)
}
func renderWeights(entries []Entry) string {
	var v []string
	for _, e := range entries {
		v = append(v, e.Path+" = "+number(e.Weight))
	}
	return "list(" + strings.Join(v, ", ") + ")"
}
func renderClimate(grid [][]string, keys []string) string {
	if len(grid) == 0 {
		return "null"
	}
	var rows []string
	for i, row := range grid {
		var cells []string
		for j, path := range row {
			cells = append(cells, "\t\t\t"+quote(MoistureKeys[j])+" = "+path)
		}
		rows = append(rows, "\t\t"+quote(keys[i])+" = list(\n"+strings.Join(cells, ",\n")+"\n\t\t)")
	}
	return "list(\n" + strings.Join(rows, ",\n") + "\n\t)"
}

func (p *Project) Changes() ([]ship.FileChange, error) {
	if err := p.State.Validate(p.Catalog); err != nil {
		return nil, err
	}
	if !p.Modified() {
		return nil, nil
	}
	// Generated types are private to this project. A handwritten type with the
	// same identifier must never be overridden by a newly generated definition.
	var owned []string
	if p.managesEnvironment() {
		owned = append(owned, p.environmentPath())
	}
	if p.managesRuins() {
		owned = append(owned, p.ruinPath())
	}
	if p.managesRivers() {
		owned = append(owned, p.riverPath())
	}
	if p.State.New || p.State.Definition.Generator == p.generatorPath() || p.State.Definition.Settings != p.saved.Definition.Settings {
		owned = append(owned, p.generatorPath())
	}
	for path, biome := range p.State.Biomes {
		if biome.Local {
			owned = append(owned, path)
		}
	}
	for _, path := range owned {
		if o := p.Catalog.Dme.Objects[path]; o != nil && !strings.EqualFold(filepath.Clean(filepath.Join(p.Catalog.Dme.RootDir, o.Location.File)), filepath.Clean(p.code)) {
			return nil, fmt.Errorf("%s is already defined outside this planet project; choose a different identifier.", path)
		}
	}
	for _, source := range p.sources {
		data, err := os.ReadFile(source.Path)
		if !source.Existed && os.IsNotExist(err) {
			continue
		}
		if err != nil || !source.Existed || !bytes.Equal(data, source.Before) {
			return nil, fmt.Errorf("%s changed outside this project; save stopped.", filepath.Base(source.Path))
		}
	}
	changes := map[string]ship.FileChange{}
	put := func(path string, data []byte) { c := p.sources[path]; c.After = data; changes[path] = c }
	current := func(path string) []byte {
		if c, ok := changes[path]; ok {
			return c.After
		}
		return p.sources[path].Before
	}
	patch := func(path, field, value string, isList bool) error {
		o := p.Catalog.Dme.Objects[path]
		if o == nil {
			return fmt.Errorf("Cannot locate source for %s.", path)
		}
		file := filepath.Join(p.Catalog.Dme.RootDir, o.Location.File)
		if _, ok := p.sources[file]; !ok {
			return fmt.Errorf("Source was not captured: %s", file)
		}
		var data []byte
		var err error
		if isList {
			data, err = ship.RewriteLiteralList(current(file), path, field, value)
		} else {
			data, err = rewritePath(current(file), path, field, value)
		}
		if err != nil {
			return err
		}
		put(file, data)
		return nil
	}
	if p.State.New {
		file := filepath.Join(p.Catalog.Dme.RootDir, markerRegistry)
		data, err := registerMarker(current(file), p.markerPath())
		if err != nil {
			return nil, err
		}
		put(file, data)
	}
	if !p.State.New {
		if p.managesEnvironment() && p.State.Definition.EnvironmentPath != p.environmentPath() {
			if err := patch(p.State.Definition.Path, "environment", p.environmentPath(), false); err != nil {
				return nil, err
			}
		}
		if p.managesRuins() && p.State.Definition.RuinPath != p.ruinPath() {
			if err := patch(p.State.Definition.Path, "ruin_settings", p.ruinPath(), false); err != nil {
				return nil, err
			}
		}
		if p.State.Definition.GameSize != p.saved.Definition.GameSize {
			if err := patch(p.State.Definition.Path, "planet_size", fmt.Sprint(p.State.Definition.GameSize), false); err != nil {
				return nil, err
			}
		}
		if p.managesRivers() && p.State.Definition.RiverPath != p.riverPath() {
			if err := patch(p.State.Definition.Path, "river_settings", p.riverPath(), false); err != nil {
				return nil, err
			}
		}
		if !reflect.DeepEqual(p.State.Definition.Surface, p.saved.Definition.Surface) {
			if err := patch(p.State.Definition.Path, "overworld_biomes", renderClimate(p.State.Definition.Surface, HeatKeys), true); err != nil {
				return nil, err
			}
		}
		if !reflect.DeepEqual(p.State.Definition.Caves, p.saved.Definition.Caves) {
			if err := patch(p.State.Definition.Path, "cave_biomes", renderClimate(p.State.Definition.Caves, CaveKeys), true); err != nil {
				return nil, err
			}
		}
		if p.State.Definition.Settings != p.saved.Definition.Settings {
			if p.State.Definition.Schema > 0 {
				if err := patch(p.State.Definition.Path, "terrain_generator", p.generatorPath(), false); err != nil {
					return nil, err
				}
			} else {
				if p.State.Definition.Overmap == "" {
					return nil, fmt.Errorf("This planet has no overmap registration for terrain settings.")
				}
				if err := patch(p.State.Definition.Overmap, "mapgen", p.generatorPath(), false); err != nil {
					return nil, err
				}
				for path, o := range p.Catalog.Dme.Objects {
					if strings.HasPrefix(path, "/area/") && o.Vars.ValueV("planet_type", "") == p.State.Definition.Path {
						if err := patch(path, "map_generator", p.generatorPath(), false); err != nil {
							return nil, err
						}
					}
				}
			}
		}
	}
	put(p.code, p.generated())
	data, err := json.MarshalIndent(p.State, "", "  ")
	if err != nil {
		return nil, err
	}
	put(p.meta, append(data, '\n'))
	put(p.Catalog.Dme.RootFile, ship.AddInclude(current(p.Catalog.Dme.RootFile), p.Catalog.Dme.RootDir, p.code))
	var out []ship.FileChange
	for _, c := range changes {
		if !c.Existed || !bytes.Equal(c.Before, c.After) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// Add a new planet to the existing dynamic spawn list without replacing the
// setup proc or changing any existing planet's spawn behavior.
func registerMarker(data []byte, path string) ([]byte, error) {
	mask := ship.SourceMask(data, true)
	head := regexp.MustCompile(`(?m)^/datum/controller/subsystem/overmap/proc/setup_planets\(\)[\t ]*\r?$`).FindIndex(mask)
	if head == nil {
		return nil, fmt.Errorf("Cannot find the project's planet spawn registry.")
	}
	end := len(mask)
	if next := regexp.MustCompile(`(?m)^/`).FindIndex(mask[head[1]:]); next != nil {
		end = head[1] + next[0]
	}
	assign := regexp.MustCompile(`var/list/dynamic_planet_markers\s*=\s*list\(`).FindAllIndex(mask[head[1]:end], -1)
	if len(assign) != 1 {
		return nil, fmt.Errorf("The planet spawn registry needs one literal dynamic_planet_markers list.")
	}
	a := head[1] + assign[0][1]
	b := a
	for b < end && mask[b] != ')' {
		if mask[b] == '(' {
			return nil, fmt.Errorf("The planet spawn registry contains computed entries.")
		}
		b++
	}
	if b == end {
		return nil, fmt.Errorf("The planet spawn registry list is incomplete.")
	}
	items, err := split(string(mask[a:b]), ',')
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item == path {
			return data, nil
		}
		if !regexp.MustCompile(`^/obj/structure/overmap/planet/[A-Za-z0-9_/]+$`).MatchString(item) {
			return nil, fmt.Errorf("The planet spawn registry contains computed entries.")
		}
	}
	newline := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		newline = "\r\n"
	}
	prefix := ""
	if text := strings.TrimSpace(string(mask[a:b])); text != "" && !strings.HasSuffix(text, ",") {
		prefix = ","
	}
	insert := prefix + newline + "\t\t" + path + "," + newline + "\t"
	return append(append(append([]byte{}, data[:b]...), []byte(insert)...), data[b:]...), nil
}
func (p *Project) Save() error {
	changes, err := p.Changes()
	if err != nil {
		return err
	}
	if err = ship.WriteChanges(p.Catalog.Dme.RootDir, changes); err != nil {
		return err
	}
	if p.State.Definition.Settings != p.saved.Definition.Settings || p.State.New {
		p.State.Definition.Generator = p.generatorPath()
	}
	if p.managesRivers() {
		p.State.Definition.RiverPath = p.riverPath()
	}
	if p.managesEnvironment() {
		p.State.Definition.EnvironmentPath = p.environmentPath()
	}
	if p.managesRuins() {
		p.State.Definition.RuinPath = p.ruinPath()
	}
	for _, c := range changes {
		c.Before, c.After, c.Existed = c.After, nil, true
		p.sources[c.Path] = c
	}
	p.saved = Clone(p.State)
	return nil
}

// Only typepath-valued assignments are allowed here; computed registrations are
// rejected instead of deleting code. Comments and neighboring variables survive.
func rewritePath(data []byte, path, field, value string) ([]byte, error) {
	mask := ship.SourceMask(data, true)
	headers := regexp.MustCompile(`(?m)^/[^\r\n]*`).FindAllIndex(mask, -1)
	start, end, count := -1, len(data), 0
	for _, h := range headers {
		if strings.TrimSpace(string(mask[h[0]:h[1]])) == path {
			start = h[1]
			count++
		} else if start >= 0 && end == len(data) {
			end = h[0]
		}
	}
	if count != 1 || end < start {
		return nil, fmt.Errorf("Cannot uniquely locate %s.", path)
	}
	block := mask[start:end]
	if regexp.MustCompile(`(?m)^[\t ]*#`).Match(block) {
		return nil, fmt.Errorf("%s has conditional definitions.", path)
	}
	re := regexp.MustCompile(`(?m)^[\t ]+` + regexp.QuoteMeta(field) + `[\t ]*=[\t ]*`)
	matches := re.FindAllIndex(block, -1)
	if len(matches) > 1 {
		return nil, fmt.Errorf("Multiple %s assignments.", field)
	}
	newline := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		newline = "\r\n"
	}
	if len(matches) == 0 {
		at := start
		for at < len(data) && (data[at] == '\r' || data[at] == '\n') {
			at++
		}
		return append(append(append([]byte{}, data[:at]...), []byte("\t"+field+" = "+value+newline)...), data[at:]...), nil
	}
	a := start + matches[0][1]
	lineEnd := a
	for lineEnd < end && mask[lineEnd] != '\n' {
		lineEnd++
	}
	old := strings.TrimSpace(string(mask[a:lineEnd]))
	if !regexp.MustCompile(`^(?:/[A-Za-z0-9_/]+|null|[0-9]+)$`).MatchString(old) {
		return nil, fmt.Errorf("%s has a computed %s.", path, field)
	}
	return append(append(append([]byte{}, data[:a]...), []byte(value)...), data[a+len(old):]...), nil
}
