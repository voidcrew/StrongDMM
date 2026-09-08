package ship

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmmsave"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

type Document struct {
	Active      bool
	Map         *dmmap.Dmm
	Initial     *dmmdata.DmmData
	Before      []byte
	Existed     bool
	fingerprint [32]byte
}

type Settings struct {
	Version         int
	ID, Description string
	Hull            Hull
	Crew            int
	Hidden          bool
	Cost            int
	PortDirection   int
}

type Project struct {
	BeforeOpen     func(string) error
	Catalog        *Catalog
	Dme            *dmenv.Dme
	Hull           Hull
	Settings       *Settings // nil for hand-authored registrations
	Documents      map[string]*Document
	files          map[string]FileChange
	savedSettings  []byte
	RoomAreas      []RoomArea
	savedAreas     []byte
	draftAreaPaths map[string]bool
	rooms          *roomEditing
	savedRooms     []byte
	Crew           *CrewConfig
	savedCrew      []byte
	crewOriginal   map[string][]CrewJob
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,47}$`)

func ValidID(id string) error {
	if !identifier.MatchString(id) {
		return fmt.Errorf("use a lowercase ID starting with a letter (letters, digits, underscores; up to 48 characters)")
	}
	return nil
}

// RawData preserves native tile order and explicit prefab overrides.
func RawData(m *dmmap.Dmm) *dmmdata.DmmData {
	d := &dmmdata.DmmData{Filepath: m.Path.Absolute, IsTgm: true, LineBreak: "\n", KeyLength: 4, MaxX: m.MaxX, MaxY: m.MaxY, MaxZ: m.MaxZ, Dictionary: dmmdata.DataDictionary{}, Grid: dmmdata.DataGrid{}}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	for n, tile := range m.Tiles {
		key := []byte("aaaa")
		v := n
		for i := 3; i >= 0; i-- {
			key[i] = alphabet[v%52]
			v /= 52
		}
		k := dmmdata.Key(key)
		d.Dictionary[k] = tile.Instances().Prefabs()
		d.Grid[tile.Coord] = k
	}
	return d
}

func fingerprint(m *dmmap.Dmm) [32]byte { return sha256.Sum256(RawData(m).EncodeTGM()) }
func (d *Document) Modified() bool {
	return d.Active && (!d.Existed || fingerprint(d.Map) != d.fingerprint)
}

func OpenProject(c *Catalog, dme *dmenv.Dme, h Hull) (*Project, error) {
	p := &Project{Catalog: c, Dme: dme, Hull: h, Documents: map[string]*Document{}, files: map[string]FileChange{}}
	path, _ := Inside(c.Root, "voidcrew/mapping/ship_projects/"+strings.TrimPrefix(h.Type, HullType+"/")+".ship.json")
	data, err := os.ReadFile(path)
	if err == nil {
		var s Settings
		if err = json.Unmarshal(data, &s); err != nil {
			return nil, err
		}
		if s.Version != 1 || s.Hull.Type != h.Type {
			return nil, fmt.Errorf("unsupported ship project %s", path)
		}
		if err := ValidID(s.ID); err != nil {
			return nil, err
		}
		if s.Hull.Type != HullType+"/"+s.ID {
			return nil, fmt.Errorf("project ID does not match its template")
		}
		p.Settings = &s
		p.Hull = s.Hull
		if err = p.installTypes(); err != nil {
			return nil, err
		}
		for _, file := range p.outputPaths() {
			if err = p.track(file); err != nil {
				return nil, err
			}
		}
		p.savedSettings = p.settingsBytes()
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err = p.openAreas(); err != nil {
		return nil, err
	}
	p.savedRooms = p.roomBytes()
	if err = p.openCrew(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Project) document(file string) (*Document, error) {
	if d := p.Documents[file]; d != nil && (d.Active || d.Existed) {
		d.Active = true
		return d, nil
	}
	if p.BeforeOpen != nil {
		if err := p.BeforeOpen(file); err != nil {
			return nil, err
		}
	}
	before, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	data, err := dmmdata.New(file)
	if err != nil {
		return nil, err
	}
	m, unknown := dmmap.New(p.Dme, data, "")
	if len(unknown) > 0 {
		return nil, fmt.Errorf("%s has %d unknown types; reload the environment before editing", file, len(unknown))
	}
	p.reserveAnchors(m)
	p.protect(m)
	d := &Document{Map: m, Initial: data, Before: before, Existed: true, Active: true, fingerprint: fingerprint(m)}
	p.Documents[file] = d
	return d, nil
}

func (p *Project) protect(m *dmmap.Dmm) {
	for _, tile := range m.Tiles {
		tile.DefaultTurf = dmmap.PrefabStorage.Initial("/turf/template_noop")
		tile.DefaultArea = dmmap.PrefabStorage.Initial("/area/template_noop")
		tile.InstancesRegenerate()
	}
}
func (p *Project) reserveAnchors(m *dmmap.Dmm) {
	for _, tile := range m.Tiles {
		tile.Reserved = nil
		for _, i := range tile.Instances() {
			if mappingMarker(i.Prefab().Path()) {
				tile.Reserved = append(tile.Reserved, i.Prefab())
			}
		}
	}
}

func (p *Project) moduleFile(m Module, theme string) (string, error) {
	themed, err := Inside(p.Catalog.Root, filepath.Join(p.Catalog.ModuleDir, strings.TrimSuffix(m.File, ".dmm")+"_"+theme+".dmm"))
	if err != nil {
		return "", err
	}
	if theme != "" && p.Documents[themed] != nil && p.Documents[themed].Active {
		return themed, nil
	}
	base, err := Inside(p.Catalog.Root, filepath.Join(p.Catalog.ModuleDir, m.File))
	if err != nil {
		return "", err
	}
	if theme == "" && p.Documents[base] != nil && p.Documents[base].Active {
		return base, nil
	}
	return p.Catalog.ModuleFile(m, theme)
}

func (p *Project) Assemble(theme Theme, selected map[string]string) (*Assembly, error) {
	file, err := p.Catalog.HullFile(p.Hull, theme)
	if err != nil {
		return nil, err
	}
	d, err := p.document(file)
	if err != nil {
		return nil, err
	}
	p.protect(d.Map)
	sources := []Source{{Name: "Hull", File: file, Data: RawData(d.Map), Live: d.Map}}
	for _, slot := range p.Hull.SlotsFor(theme) {
		id := selected[slot]
		if id == "" {
			continue
		}
		found := false
		for _, m := range p.Hull.Modules {
			if m.ID != id || m.Slot != slot || !m.Available(theme.ID) {
				continue
			}
			found = true
			file, err := p.moduleFile(m, theme.ID)
			if err != nil {
				return nil, err
			}
			d, err := p.document(file)
			if err != nil {
				return nil, err
			}
			p.protect(d.Map)
			sources = append(sources, Source{Name: m.Name, Slot: slot, File: file, Data: RawData(d.Map), Live: d.Map})
		}
		if !found {
			return nil, fmt.Errorf("module %s is unavailable", id)
		}
	}
	a, err := Compose(sources)
	if err != nil {
		return nil, err
	}
	for _, slot := range p.Hull.SlotsFor(theme) {
		if _, ok := a.Markers[slot]; !ok {
			a.Issues = append(a.Issues, Issue{Message: "Missing hull marker: " + slot})
		}
	}
	a.CheckHull()
	a.CheckAccess(p.Dme)
	return a, nil
}

func (p *Project) addMap(file string, data *dmmdata.DmmData) error {
	if p.Documents[file] != nil && p.Documents[file].Active {
		return fmt.Errorf("map already exists: %s", file)
	}
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("file already exists: %s", file)
	} else if !os.IsNotExist(err) {
		return err
	}
	data.Filepath = file
	m, unknown := dmmap.New(p.Dme, data, "")
	if len(unknown) > 0 {
		return fmt.Errorf("new map references %d unknown types", len(unknown))
	}
	p.reserveAnchors(m)
	p.protect(m)
	if prior := p.Documents[file]; prior != nil {
		*prior.Map = *m
		prior.Active = true
	} else {
		p.Documents[file] = &Document{Map: m, Active: true}
	}
	return nil
}

func NewProject(c *Catalog, dme *dmenv.Dme, id, name string, width, height int) (*Project, error) {
	if err := ValidID(id); err != nil {
		return nil, err
	}
	if err := ShipNameError(c, dme, name, ""); err != nil {
		return nil, err
	}
	if width < 5 || height < 5 || width > 128 || height > 128 {
		return nil, fmt.Errorf("canvas dimensions must be between 5 and 128")
	}
	h := Hull{Type: HullType + "/" + id, Name: name, Prefix: "_maps/voidcrew/ships/", Port: "ship", Suffix: id, Themes: []Theme{{ID: "standard", Name: "Standard", Suffix: id, Default: true}}}
	if dme.Objects[h.Type] != nil {
		return nil, fmt.Errorf("ship ID already registered: %s", id)
	}
	for _, existing := range c.Hulls {
		if existing.Type == h.Type {
			return nil, fmt.Errorf("ship ID already open: %s", id)
		}
	}
	p := &Project{Catalog: c, Dme: dme, Hull: h, Settings: &Settings{Version: 1, ID: id, Crew: 4, Hidden: true, PortDirection: 2}, Documents: map[string]*Document{}, files: map[string]FileChange{}}
	for _, file := range p.outputPaths() {
		if file == dme.RootFile {
			continue
		}
		if _, err := os.Stat(file); err == nil {
			return nil, fmt.Errorf("ship output already exists: %s", file)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if err := p.installTypes(); err != nil {
		return nil, err
	}
	file, err := c.HullFile(h, h.Themes[0])
	if err != nil {
		return nil, err
	}
	data := blankData(width, height)
	// A port supplies the ship identity. Everything else starts as empty canvas.
	coord := util.Point{X: width / 2, Y: 1, Z: 1}
	data.Dictionary["aab"] = append(dmmdata.Prefabs{p.prefab(p.portType(), map[string]string{"dir": "1"})}, data.Dictionary["aaa"]...)
	data.Grid[coord] = "aab"
	if err = p.addMap(file, data); err != nil {
		return nil, err
	}
	for _, file := range p.outputPaths() {
		if err = p.track(file); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func blankData(w, h int) *dmmdata.DmmData {
	d := &dmmdata.DmmData{KeyLength: 3, IsTgm: true, LineBreak: "\n", MaxX: w, MaxY: h, MaxZ: 1, Dictionary: dmmdata.DataDictionary{"aaa": {dmmap.PrefabStorage.Initial("/turf/template_noop"), dmmap.PrefabStorage.Initial("/area/template_noop")}}, Grid: dmmdata.DataGrid{}}
	for y := 1; y <= h; y++ {
		for x := 1; x <= w; x++ {
			d.Grid[util.Point{X: x, Y: y, Z: 1}] = "aaa"
		}
	}
	return d
}

func (p *Project) areaType() string { return "/area/shuttle/voidcrew/" + p.Settings.ID }
func (p *Project) portType() string { return "/obj/docking_port/mobile/voidcrew/" + p.Settings.ID }
func (p *Project) installTypes() error {
	s := p.Settings
	if err := p.Dme.AddDraftType(p.areaType(), map[string]string{"name": dmQuote(p.Hull.Name), "icon_state": `"station"`}); err != nil {
		return err
	}
	return p.Dme.AddDraftType(p.portType(), map[string]string{"name": dmQuote(p.Hull.Name), "area_type": p.areaType(), "port_direction": strconv.Itoa(s.PortDirection)})
}
func (p *Project) prefab(path string, values map[string]string) *dmmprefab.Prefab {
	vars := dmvars.MutableVariables{}
	for k, v := range values {
		vars.Put(k, v)
	}
	v := vars.ToImmutable()
	if obj := p.Dme.Objects[path]; obj != nil {
		v.LinkParent(obj.Vars)
	}
	return dmmap.PrefabStorage.Put(dmmprefab.New(0, path, v))
}

// Deck fills a selected hull rectangle with plating and the ship's own area.
func (p *Project) Deck(theme Theme, min, max util.Point) error {
	if p.Settings == nil {
		return fmt.Errorf("deck creation requires an authored ship project")
	}
	file, _ := p.Catalog.HullFile(p.Hull, theme)
	d, err := p.document(file)
	if err != nil {
		return err
	}
	if !d.Map.HasTile(min) || !d.Map.HasTile(max) {
		return fmt.Errorf("select tiles inside the hull")
	}
	for y := min.Y; y <= max.Y; y++ {
		for x := min.X; x <= max.X; x++ {
			tile := d.Map.GetTile(util.Point{X: x, Y: y, Z: 1})
			tile.InstancesRemoveByPath("/turf")
			tile.InstancesRemoveByPath("/area")
			tile.InstancesAdd(dmmap.PrefabStorage.Initial("/turf/open/floor/plating"))
			tile.InstancesAdd(dmmap.PrefabStorage.Initial(p.areaType()))
		}
	}
	return nil
}

func (p *Project) settingsBytes() []byte {
	if p.Settings == nil {
		return nil
	}
	if p.Settings == nil {
		return nil
	}
	p.Settings.Hull = p.Hull
	b, _ := json.MarshalIndent(p.Settings, "", "  ")
	return append(b, '\n')
}
func (p *Project) Modified() bool {
	if !bytes.Equal(p.crewBytes(), p.savedCrew) {
		return true
	}
	if !bytes.Equal(p.roomBytes(), p.savedRooms) {
		return true
	}
	if !bytes.Equal(p.areaBytes(), p.savedAreas) {
		return true
	}
	if !bytes.Equal(p.settingsBytes(), p.savedSettings) {
		return true
	}
	for _, d := range p.Documents {
		if d.Modified() {
			return true
		}
	}
	return false
}
func (p *Project) outputPaths() []string {
	if p.Settings == nil {
		return nil
	}
	id := p.Settings.ID
	result := []string{}
	for _, rel := range []string{"voidcrew/mapping/ship_projects/" + id + ".ship.json", "voidcrew/mapping/shuttles/" + id + ".dm", "voidcrew/modules/ship_upgrades/ships/" + id + ".dm"} {
		file, _ := Inside(p.Catalog.Root, rel)
		result = append(result, file)
	}
	return append(result, p.Dme.RootFile)
}
func (p *Project) track(path string) error {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	p.files[path] = FileChange{Path: path, Before: b, Existed: err == nil}
	return nil
}

func (p *Project) Changes() ([]FileChange, error) {
	var changes []FileChange
	for path, d := range p.Documents {
		if !d.Modified() {
			continue
		}
		for _, tile := range d.Map.Tiles {
			for _, inst := range tile.Instances() {
				if !p.AreaActive(inst.Prefab().Path()) {
					return nil, fmt.Errorf("map uses an undone area definition: %s; redo its creation or assign another area", inst.Prefab().Path())
				}
			}
		}
		data, err := dmmsave.Prepare(p.Dme, d.Map, d.Initial)
		if err != nil {
			return nil, err
		}
		changes = append(changes, FileChange{Path: path, Before: d.Before, Existed: d.Existed, After: data.EncodeTGM()})
	}
	if p.Settings != nil && (!bytes.Equal(p.settingsBytes(), p.savedSettings) || !bytes.Equal(p.crewBytes(), p.savedCrew)) {
		paths := p.outputPaths()
		hull, modules, err := p.registration()
		if err != nil {
			return nil, err
		}
		include := p.files[p.Dme.RootFile].Before
		newline := "\n"
		if bytes.Contains(include, []byte("\r\n")) {
			newline = "\r\n"
		}
		for _, path := range paths[1:3] {
			rel, _ := filepath.Rel(p.Catalog.Root, path)
			line := `#include "` + filepath.ToSlash(rel) + `"`
			if !strings.Contains(strings.ReplaceAll(string(include), "\\", "/"), line) {
				marker := []byte("// END_INCLUDE")
				if i := bytes.Index(include, marker); i >= 0 {
					include = append(append(append([]byte{}, include[:i]...), []byte(line+newline)...), include[i:]...)
				} else {
					include = append(include, []byte(newline+line+newline)...)
				}
			}
		}
		for i, data := range [][]byte{p.settingsBytes(), hull, modules, include} {
			c := p.files[paths[i]]
			c.After = data
			if !c.Existed || !bytes.Equal(c.Before, c.After) {
				changes = append(changes, c)
			}
		}
	}
	var err error
	changes, err = p.roomChanges(changes)
	if err != nil {
		return nil, err
	}
	changes, err = p.areaChanges(changes)
	if err != nil {
		return nil, err
	}
	changes, err = p.crewChanges(changes)
	if err != nil {
		return nil, err
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}

func (p *Project) Save() error {
	return SaveProjects([]*Project{p})
}
func (p *Project) accept(changes []FileChange) error {
	for _, c := range changes {
		if d := p.Documents[c.Path]; d != nil {
			d.Before = c.After
			d.Existed = true
			var err error
			d.Initial, err = dmmdata.New(c.Path)
			if err != nil {
				return err
			}
			d.fingerprint = fingerprint(d.Map)
		} else if _, tracked := p.files[c.Path]; tracked {
			c.Before = c.After
			c.Existed = true
			c.After = nil
			p.files[c.Path] = c
		}
	}
	p.savedSettings = p.settingsBytes()
	p.savedCrew = p.crewBytes()
	p.savedAreas = p.areaBytes()
	p.savedRooms = p.roomBytes()
	return nil
}

// SaveProjects also merges the shared environment includes when several ships
// are created in one session. All outputs share one preflight and rollback.
func SaveProjects(projects []*Project) error {
	if len(projects) == 0 {
		return nil
	}
	root := projects[0].Catalog.Root
	files := map[string]FileChange{}
	for _, p := range projects {
		if p.Catalog.Root != root {
			return fmt.Errorf("save projects from one environment at a time")
		}
		changes, err := p.Changes()
		if err != nil {
			return err
		}
		for _, c := range changes {
			if prior, ok := files[c.Path]; ok {
				if c.Path != p.Dme.RootFile || !bytes.Equal(c.Before, prior.Before) {
					return fmt.Errorf("conflicting save destination %s", c.Path)
				}
				merged := string(prior.After)
				newline := "\n"
				if strings.Contains(merged, "\r\n") {
					newline = "\r\n"
				}
				for _, line := range strings.Split(string(c.After), "\n") {
					line = strings.TrimSpace(line)
					if !strings.HasPrefix(line, "#include ") || strings.Contains(merged, line) {
						continue
					}
					at := strings.Index(merged, "// END_INCLUDE")
					if at >= 0 {
						merged = merged[:at] + line + newline + merged[at:]
					} else {
						merged += newline + line + newline
					}
				}
				prior.After = []byte(merged)
				files[c.Path] = prior
			} else {
				files[c.Path] = c
			}
		}
	}
	changes := []FileChange{}
	for _, c := range files {
		changes = append(changes, c)
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	if err := WriteChanges(root, changes); err != nil {
		return err
	}
	for _, p := range projects {
		if err := p.accept(changes); err != nil {
			return err
		}
	}
	return nil
}
