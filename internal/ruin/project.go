package ruin

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/ship"
	"sdmm/internal/util"
)

type Setup struct {
	Location      Location
	ID            string
	Properties    Properties
	Width, Height int
	Turf          string
	Power         int // 0: equipment needs power, 1: self-powered, 2: unpowered
	Gravity       bool
}

type Project struct {
	Catalog      *Catalog
	Template     Template
	Properties   Properties
	initial      Properties
	setup        *Setup
	source, dme  ship.FileChange
	dependencies []ship.FileChange
	assignments  map[string]string
	areaType     string
	areaValues   map[string]string
}

func exists(file string) bool { _, err := os.Stat(file); return !os.IsNotExist(err) }
func snapshot(file string) (ship.FileChange, error) {
	data, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return ship.FileChange{}, err
	}
	return ship.FileChange{Path: file, Before: data, Existed: err == nil}, nil
}

func (c *Catalog) ValidateSetup(s Setup) error {
	if err := s.Properties.Validate(); err != nil {
		return err
	}
	if err := c.NameError(s.Properties.Name, ""); err != nil {
		return err
	}
	if err := ship.ValidID(s.ID); err != nil {
		return err
	}
	found := false
	for _, loc := range c.Locations {
		if loc == s.Location {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("choose a ruin location from this project")
	}
	if c.idUsed(s.ID, s.Location) {
		return fmt.Errorf("this identifier or one of its files already exists")
	}
	if s.Width < 1 || s.Height < 1 || s.Width > 256 || s.Height > 256 {
		return fmt.Errorf("map width and height must be between 1 and 256 tiles")
	}
	if !strings.HasPrefix(s.Turf, "/turf/") || c.Dme.Objects[s.Turf] == nil {
		return fmt.Errorf("choose a starting turf available in this project")
	}
	if c.Dme.Objects["/area/ruin"] == nil {
		return fmt.Errorf("this project has no ruin area type")
	}
	if s.Power < 0 || s.Power > 2 {
		return fmt.Errorf("choose a power setting")
	}
	return nil
}

func New(c *Catalog, s Setup) (*Project, error) {
	if err := c.ValidateSetup(s); err != nil {
		return nil, err
	}
	file, err := ship.Inside(c.Dme.RootDir, s.Location.Prefix+s.ID+".dmm")
	if err != nil {
		return nil, err
	}
	t := Template{Type: s.Location.Type + "/" + s.ID, ID: s.ID, Name: s.Properties.Name, Location: s.Location.Name, File: file}
	p, err := open(c, t)
	if err != nil {
		return nil, err
	}
	p.setup = &s
	p.Properties = s.Properties
	p.initial = s.Properties
	p.assignments = s.Properties.values()
	p.assignments["id"] = quote(s.ID)
	p.assignments["prefix"] = quote(s.Location.Prefix)
	p.assignments["suffix"] = quote(s.ID + ".dmm")
	p.areaType = "/area/ruin/" + s.ID
	p.areaValues = map[string]string{"name": quote(s.Properties.Name), "requires_power": "TRUE", "always_unpowered": "FALSE", "default_gravity": "1"}
	if !s.Gravity {
		p.areaValues["default_gravity"] = "0"
	}
	if s.Power == 1 {
		p.areaValues["requires_power"] = "FALSE"
	}
	if s.Power == 2 {
		p.areaValues["always_unpowered"] = "TRUE"
	}
	return p, nil
}

func Open(c *Catalog, t Template) (*Project, error) {
	obj := c.Dme.Objects[t.Type]
	if obj == nil {
		return nil, fmt.Errorf("ruin is no longer in the loaded project")
	}
	p, err := open(c, t)
	if err != nil {
		return nil, err
	}
	p.Properties, err = ReadProperties(obj)
	if err != nil {
		return nil, err
	}
	p.initial = p.Properties
	// Watch the original definition too, even though only the separate override
	// file will be written. Custom procs and unrelated fields remain untouched.
	if obj.Location.File != "" {
		file, err := ship.Inside(c.Dme.RootDir, filepath.ToSlash(obj.Location.File))
		if err != nil {
			return nil, err
		}
		if file != p.source.Path && file != p.dme.Path {
			dep, err := snapshot(file)
			if err != nil {
				return nil, err
			}
			p.dependencies = append(p.dependencies, dep)
		}
	}
	return p, nil
}

func open(c *Catalog, t Template) (*Project, error) {
	p := &Project{Catalog: c, Template: t, assignments: map[string]string{}}
	file, err := ship.Inside(c.Dme.RootDir, c.sourceRelative(t.Type))
	if err != nil {
		return nil, err
	}
	p.source, err = snapshot(file)
	if err != nil {
		return nil, err
	}
	p.dme, err = snapshot(c.Dme.RootFile)
	if err != nil {
		return nil, err
	}
	if !p.dme.Existed {
		return nil, fmt.Errorf("project file is missing")
	}
	if p.source.Existed {
		body, _, _, err := p.managed()
		if err != nil {
			return nil, err
		}
		lines := strings.Split(strings.TrimSpace(body), "\n")
		if len(lines) == 0 || strings.TrimSpace(lines[0]) != t.Type {
			return nil, fmt.Errorf("unexpected ruin registration in %s", file)
		}
		assignment := regexp.MustCompile(`^\t([a-z_]+) = (.+)$`)
		for _, line := range lines[1:] {
			line = strings.TrimSuffix(line, "\r")
			if line == "" {
				continue
			}
			m := assignment.FindStringSubmatch(line)
			if m == nil {
				return nil, fmt.Errorf("custom code inside the workshop block in %s; move it outside the block before editing properties", file)
			}
			p.assignments[m[1]] = m[2]
		}
	}
	return p, nil
}

func (p *Project) markers() (string, string) {
	return "// BEGIN RUIN WORKSHOP: " + p.Template.Type, "// END RUIN WORKSHOP: " + p.Template.Type
}
func (p *Project) managed() (body string, start, end int, err error) {
	begin, finish := p.markers()
	s := string(p.source.Before)
	start = strings.Index(s, begin)
	end = strings.Index(s, finish)
	if start < 0 || end < start || strings.Count(s, begin) != 1 || strings.Count(s, finish) != 1 {
		return "", 0, 0, fmt.Errorf("%s already exists without a unique workshop block", p.source.Path)
	}
	return s[start+len(begin) : end], start, end + len(finish), nil
}

func definition(path string, values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	out.WriteString(path + "\n")
	for _, key := range keys {
		fmt.Fprintf(&out, "\t%s = %s\n", key, values[key])
	}
	return out.String()
}

func (p *Project) Modified() bool { return p.setup != nil || p.Properties != p.initial }

func (p *Project) Changes() ([]ship.FileChange, error) {
	if !p.Modified() {
		return nil, nil
	}
	if err := p.Properties.Validate(); err != nil {
		return nil, err
	}
	// A parent edited in another workshop project can change this ruin's
	// inherited settings while this form is open.
	if p.setup == nil {
		current, err := ReadProperties(p.Catalog.Dme.Objects[p.Template.Type])
		if err != nil || current != p.initial {
			return nil, fmt.Errorf("this ruin's inherited properties changed; reopen its properties before saving")
		}
	}
	if p.setup != nil || normalized(p.Properties.Name) != normalized(p.initial.Name) {
		if err := p.Catalog.NameError(p.Properties.Name, p.Template.Type); err != nil {
			return nil, err
		}
	}
	values := map[string]string{}
	for k, v := range p.assignments {
		values[k] = v
	}
	initial := p.initial.values()
	for k, v := range p.Properties.values() {
		if v != initial[k] {
			values[k] = v
		}
	}
	begin, end := p.markers()
	block := begin + "\n" + definition(p.Template.Type, values) + end
	source := p.source
	if source.Existed {
		_, start, finish, err := p.managed()
		if err != nil {
			return nil, err
		}
		if bytes.Contains(source.Before, []byte("\r\n")) {
			block = strings.ReplaceAll(block, "\n", "\r\n")
		}
		source.After = []byte(string(source.Before[:start]) + block + string(source.Before[finish:]))
	} else {
		prefix := ""
		if p.setup != nil {
			prefix = definition(p.areaType, p.areaValues) + "\n"
		}
		source.After = []byte(prefix + block + "\n")
	}
	dme := p.dme
	rel, err := filepath.Rel(p.Catalog.Dme.RootDir, source.Path)
	if err != nil {
		return nil, err
	}
	dme.After, err = include(dme.Before, filepath.ToSlash(rel))
	if err != nil {
		return nil, err
	}
	changes := []ship.FileChange{source, dme}
	for _, dep := range p.dependencies {
		dep.After = dep.Before
		changes = append(changes, dep)
	}
	if p.setup != nil {
		if p.Catalog.idUsed(p.setup.ID, p.setup.Location) {
			return nil, fmt.Errorf("the ruin identifier or its files are now in use")
		}
		s := p.setup
		prefab := func(path string) *dmmprefab.Prefab { return dmmprefab.New(0, path, &dmvars.Variables{}) }
		d := &dmmdata.DmmData{IsTgm: true, LineBreak: "\n", KeyLength: 3, MaxX: s.Width, MaxY: s.Height, MaxZ: 1, Dictionary: dmmdata.DataDictionary{"aaa": {prefab(s.Turf), prefab(p.areaType)}}, Grid: dmmdata.DataGrid{}}
		for x := 1; x <= s.Width; x++ {
			for y := 1; y <= s.Height; y++ {
				d.Grid[util.Point{X: x, Y: y, Z: 1}] = "aaa"
			}
		}
		changes = append(changes, ship.FileChange{Path: p.Template.File, After: d.EncodeTGM()})
	}
	return changes, nil
}

// Include after the project's existing definitions, preserving its line endings.
func include(before []byte, relative string) ([]byte, error) {
	s := string(before)
	pattern := regexp.MustCompile(`(?m)^\s*#include\s+"([^"]+)"`)
	for _, m := range pattern.FindAllStringSubmatch(s, -1) {
		if strings.EqualFold(strings.ReplaceAll(m[1], "\\", "/"), relative) {
			return before, nil
		}
	}
	newline := "\n"
	if strings.Contains(s, "\r\n") {
		newline = "\r\n"
	}
	line := "#include \"" + strings.ReplaceAll(relative, "/", "\\") + "\"" + newline
	marker := regexp.MustCompile(`(?m)^// END_INCLUDE[^\r\n]*`).FindStringIndex(s)
	if marker != nil {
		return []byte(s[:marker[0]] + line + s[marker[0]:]), nil
	}
	if !strings.HasSuffix(s, "\n") {
		s += newline
	}
	return []byte(s + line), nil
}

func (p *Project) Save() error {
	changes, err := p.Changes()
	if err != nil || len(changes) == 0 {
		return err
	}
	if err = ship.WriteChanges(p.Catalog.Dme.RootDir, changes); err != nil {
		return err
	}
	dme := p.Catalog.Dme
	if p.setup != nil {
		// Parents were validated before writing, so these do not require a reload.
		if err = dme.AddDraftType(p.areaType, p.areaValues); err != nil {
			return err
		}
		if err = dme.AddDraftType(p.Template.Type, p.assignments); err != nil {
			return err
		}
	}
	obj := dme.Objects[p.Template.Type]
	old := p.initial.values()
	for key, value := range p.Properties.values() {
		if value != old[key] {
			*obj.Vars = *dmvars.Set(obj.Vars, key, value)
			p.assignments[key] = value
		}
	}
	p.Template.Name = p.Properties.Name
	p.initial = p.Properties
	p.setup = nil
	for _, c := range changes {
		c.Before = c.After
		c.Existed = true
		if c.Path == p.source.Path {
			p.source = c
		}
		if c.Path == p.dme.Path {
			p.dme = c
		}
	}
	return nil
}
