package ship

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/util"
)

// RoomArea is a real DM subtype, shared by this ship's themes and room options.
type RoomArea struct {
	Path, Name, IconState string
}

type areaSettings struct {
	Version int
	Ship    string
	Areas   []RoomArea
}

func (p *Project) areaPaths() (string, string, error) {
	id := strings.ReplaceAll(strings.TrimPrefix(p.Hull.Type, HullType+"/"), "/", "__")
	if err := ValidID(id); err != nil {
		return "", "", err
	}
	meta, err := Inside(p.Catalog.Root, "voidcrew/mapping/ship_projects/"+id+".areas.json")
	if err != nil {
		return "", "", err
	}
	code, err := Inside(p.Catalog.Root, "voidcrew/mapping/ship_areas/"+id+".dm")
	return meta, code, err
}

func (p *Project) areaBytes() []byte {
	if len(p.RoomAreas) == 0 {
		return nil
	}
	b, _ := json.MarshalIndent(areaSettings{1, p.Hull.Type, p.RoomAreas}, "", "  ")
	return append(b, '\n')
}

func (p *Project) openAreas() error {
	p.draftAreaPaths = map[string]bool{}
	meta, code, err := p.areaPaths()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(meta)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var settings areaSettings
	if err = json.Unmarshal(b, &settings); err != nil {
		return err
	}
	if settings.Version != 1 || settings.Ship != p.Hull.Type {
		return fmt.Errorf("area definitions belong to a different ship or project version")
	}
	seen := map[string]bool{}
	for _, area := range settings.Areas {
		if err = validateRoomArea(area); err != nil {
			return err
		}
		if seen[area.Path] {
			return fmt.Errorf("duplicate ship area: %s", area.Path)
		}
		seen[area.Path] = true
		if err = p.Dme.AddDraftType(area.Path, areaValues(area)); err != nil {
			return err
		}
		p.draftAreaPaths[area.Path] = true
	}
	p.RoomAreas = settings.Areas
	p.savedAreas = p.areaBytes()
	for _, file := range []string{meta, code, p.Dme.RootFile} {
		if _, ok := p.files[file]; !ok {
			if err = p.track(file); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRoomArea(area RoomArea) error {
	if !strings.HasPrefix(area.Path, "/area/shuttle/voidcrew/") || strings.Contains(area.Path, "..") {
		return fmt.Errorf("area must inherit from a Voidcrew ship area")
	}
	for _, id := range strings.Split(strings.TrimPrefix(area.Path, "/area/shuttle/voidcrew/"), "/") {
		if err := ValidID(id); err != nil {
			return err
		}
	}
	if strings.TrimSpace(area.Name) == "" {
		return fmt.Errorf("enter an area name")
	}
	return nil
}

func areaValues(area RoomArea) map[string]string {
	return map[string]string{"name": dmQuote(area.Name), "icon_state": dmQuote(area.IconState)}
}

// Resolve the real mobile port's area_type; a ship's file ID need not match it.
func (p *Project) AreaRoot(theme Theme) (string, error) {
	if p.Settings != nil {
		return p.areaType(), nil
	}
	file, err := p.Catalog.HullFile(p.Hull, theme)
	if err != nil {
		return "", err
	}
	d, err := p.document(file)
	if err != nil {
		return "", err
	}
	root, ports := "", 0
	for _, tile := range d.Map.Tiles {
		for _, inst := range tile.Instances() {
			if strings.HasPrefix(inst.Prefab().Path(), "/obj/docking_port/mobile/") {
				ports++
				root = inst.Prefab().Vars().ValueV("area_type", "")
			}
		}
	}
	if ports != 1 || !strings.HasPrefix(root, "/area/shuttle/voidcrew/") || p.Dme.Objects[root] == nil {
		return "", fmt.Errorf("the hull needs one mobile port with a ship-specific area_type")
	}
	return root, nil
}

func (p *Project) AreaIDUsed(root, id string) bool { return p.Dme.Objects[root+"/"+id] != nil }

// Draft types remain cached for redo, but must not be saved after creation is undone.
func (p *Project) AreaActive(path string) bool {
	if !p.draftAreaPaths[path] {
		return true
	}
	for _, area := range p.RoomAreas {
		if area.Path == path {
			return true
		}
	}
	return false
}

func (p *Project) AddArea(theme Theme, id, name, icon string) (string, error) {
	if err := p.AreaNameError(theme, name); err != nil {
		return "", err
	}
	if err := ValidID(id); err != nil {
		return "", err
	}
	root, err := p.AreaRoot(theme)
	if err != nil {
		return "", err
	}
	area := RoomArea{root + "/" + id, strings.TrimSpace(name), icon}
	if err = validateRoomArea(area); err != nil {
		return "", err
	}
	if p.AreaIDUsed(root, id) {
		return "", fmt.Errorf("area identifier is already defined")
	}
	meta, code, err := p.areaPaths()
	if err != nil {
		return "", err
	}
	if _, tracked := p.files[meta]; !tracked {
		for _, path := range []string{meta, code} {
			if _, err = os.Stat(path); !os.IsNotExist(err) {
				return "", fmt.Errorf("area output already exists or cannot be checked: %s", path)
			}
		}
		for _, path := range []string{meta, code, p.Dme.RootFile} {
			if _, ok := p.files[path]; !ok {
				if err = p.track(path); err != nil {
					return "", err
				}
			}
		}
	}
	if err = p.Dme.AddDraftType(area.Path, areaValues(area)); err != nil {
		return "", err
	}
	if p.draftAreaPaths == nil {
		p.draftAreaPaths = map[string]bool{}
	}
	p.draftAreaPaths[area.Path] = true
	p.RoomAreas = append(p.RoomAreas, area)
	return area.Path, nil
}

func (p *Project) Areas(theme Theme) ([]RoomArea, error) {
	root, err := p.AreaRoot(theme)
	if err != nil {
		return nil, err
	}
	active := map[string]bool{}
	for _, area := range p.RoomAreas {
		active[area.Path] = true
	}
	var areas []RoomArea
	for path, obj := range p.Dme.Objects {
		if path != root && !strings.HasPrefix(path, root+"/") {
			continue
		}
		if p.draftAreaPaths[path] && !active[path] {
			continue
		}
		areas = append(areas, RoomArea{path, text(obj.Vars, "name"), text(obj.Vars, "icon_state")})
	}
	sort.Slice(areas, func(i, j int) bool {
		if areas[i].Name == areas[j].Name {
			return areas[i].Path < areas[j].Path
		}
		return areas[i].Name < areas[j].Name
	})
	return areas, nil
}

// Bounds are local to the selected source, including when editing a placed module.
// Only /area changes: floors, walls, objects and slot markers are preserved.
func (p *Project) AssignArea(theme Theme, file, path string, lo, hi util.Point) error {
	areas, err := p.Areas(theme)
	if err != nil {
		return err
	}
	found := false
	for _, area := range areas {
		if area.Path == path {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("choose an area belonging to this ship")
	}
	d := p.Documents[file]
	if d == nil || !d.Active {
		return fmt.Errorf("select an open ship part")
	}
	if lo.Z != 1 || hi.Z != 1 || lo.X > hi.X || lo.Y > hi.Y || !d.Map.HasTile(lo) || !d.Map.HasTile(hi) {
		return fmt.Errorf("select tiles inside the part being edited")
	}
	for y := lo.Y; y <= hi.Y; y++ {
		for x := lo.X; x <= hi.X; x++ {
			tile := d.Map.GetTile(util.Point{X: x, Y: y, Z: 1})
			tile.InstancesRemoveByPath("/area")
			tile.InstancesAdd(dmmap.PrefabStorage.Initial(path))
		}
	}
	return nil
}

func addInclude(content []byte, root, file string) []byte {
	rel, _ := filepath.Rel(root, file)
	line := `#include "` + filepath.ToSlash(rel) + `"`
	if strings.Contains(strings.ReplaceAll(string(content), "\\", "/"), line) {
		return content
	}
	newline := "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		newline = "\r\n"
	}
	if i := bytes.Index(content, []byte("// END_INCLUDE")); i >= 0 {
		return append(append(append([]byte{}, content[:i]...), []byte(line+newline)...), content[i:]...)
	}
	return append(append([]byte{}, content...), []byte(newline+line+newline)...)
}

func (p *Project) areaChanges(changes []FileChange) ([]FileChange, error) {
	if bytes.Equal(p.areaBytes(), p.savedAreas) {
		return changes, nil
	}
	meta, code, err := p.areaPaths()
	if err != nil {
		return nil, err
	}
	var definitions strings.Builder
	for _, area := range p.RoomAreas {
		if err = validateRoomArea(area); err != nil {
			return nil, err
		}
		fmt.Fprintf(&definitions, "%s\n\tname = %s\n\ticon_state = %s\n\n", area.Path, dmQuote(area.Name), dmQuote(area.IconState))
	}
	metadata := p.areaBytes()
	if metadata == nil {
		metadata, _ = json.MarshalIndent(areaSettings{1, p.Hull.Type, []RoomArea{}}, "", "  ")
	}
	for path, content := range map[string][]byte{meta: metadata, code: []byte(definitions.String())} {
		c := p.files[path]
		c.After = content
		changes = append(changes, c)
	}
	for i, c := range changes {
		if c.Path == p.Dme.RootFile {
			changes[i].After = addInclude(c.After, p.Catalog.Root, code)
			return changes, nil
		}
	}
	c := p.files[p.Dme.RootFile]
	c.After = addInclude(c.Before, p.Catalog.Root, code)
	if !bytes.Equal(c.Before, c.After) {
		changes = append(changes, c)
	}
	return changes, nil
}
