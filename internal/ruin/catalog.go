// Package ruin discovers and authors ruin templates in a loaded mapping project.
package ruin

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/ship"
)

const Type = "/datum/map_template/ruin"

type Location struct{ Type, Name, Prefix string }
type Template struct{ Type, ID, Name, Location, File, Problem string }
type Catalog struct {
	Dme       *dmenv.Dme
	Locations []Location
	Templates []Template
}

func dmText(value string) string {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return ""
	}
	value = value[1 : len(value)-1]
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] == '\\' && i+1 < len(value) {
			i++
			switch value[i] {
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case '\\', '"', '[', ']':
				out.WriteByte(value[i])
			default:
				out.WriteByte('\\')
				out.WriteByte(value[i])
			}
		} else {
			out.WriteByte(value[i])
		}
	}
	return out.String()
}
func quote(value string) string {
	r := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "[", "\\[", "\r", "", "\n", "\\n", "\t", "\\t")
	return "\"" + r.Replace(value) + "\""
}
func text(v *dmvars.Variables, key string) string { return dmText(v.ValueV(key, "null")) }

func Discover(dme *dmenv.Dme) (*Catalog, error) {
	if dme == nil || dme.Objects[Type] == nil {
		return nil, fmt.Errorf("open a project with ruin templates to use Ruin Workshop")
	}
	c := &Catalog{Dme: dme}
	labels := map[string]string{"space": "Space", "lavaland": "Lavaland", "icemoon": "Ice moon surface", "icemoon/underground": "Ice moon underground", "jungle": "Jungle", "beach": "Beach", "wasteland": "Wasteland", "reebe": "Reebe"}
	for path, obj := range dme.Objects {
		if !strings.HasPrefix(path, Type+"/") {
			continue
		}
		prefix := strings.ReplaceAll(text(obj.Vars, "prefix"), "\\", "/")
		if prefix == "" || text(obj.Vars, "suffix") != "" {
			continue
		}
		if _, err := ship.Inside(dme.RootDir, prefix); err != nil {
			continue
		}
		key := strings.TrimPrefix(path, Type+"/")
		name := labels[key]
		if name == "" {
			name = strings.ReplaceAll(strings.ReplaceAll(key, "_", " "), "/", " / ")
			words := strings.Fields(name)
			for i, word := range words {
				if len(word) > 0 {
					words[i] = strings.ToUpper(word[:1]) + word[1:]
				}
			}
			name = strings.Join(words, " ")
		}
		c.Locations = append(c.Locations, Location{Type: path, Name: name, Prefix: prefix})
	}
	sort.Slice(c.Locations, func(i, j int) bool { return c.Locations[i].Name < c.Locations[j].Name })
	for path, obj := range dme.Objects {
		if !strings.HasPrefix(path, Type+"/") || text(obj.Vars, "suffix") == "" {
			continue
		}
		t := Template{Type: path, ID: text(obj.Vars, "id"), Name: text(obj.Vars, "name")}
		if t.Name == "" {
			t.Name = t.ID
		}
		if t.Name == "" {
			t.Name = path[strings.LastIndex(path, "/")+1:]
		}
		longest := 0
		for _, loc := range c.Locations {
			if strings.HasPrefix(path, loc.Type+"/") && len(loc.Type) > longest {
				t.Location = loc.Name
				longest = len(loc.Type)
			}
		}
		var err error
		t.File, err = ship.Inside(dme.RootDir, strings.ReplaceAll(text(obj.Vars, "prefix")+text(obj.Vars, "suffix"), "\\", "/"))
		if err != nil {
			t.Problem = err.Error()
		}
		c.Templates = append(c.Templates, t)
	}
	sort.Slice(c.Templates, func(i, j int) bool {
		if c.Templates[i].Name == c.Templates[j].Name {
			return c.Templates[i].Type < c.Templates[j].Type
		}
		return strings.ToLower(c.Templates[i].Name) < strings.ToLower(c.Templates[j].Name)
	})
	return c, nil
}

func normalized(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func (c *Catalog) NameError(name, except string) error {
	if normalized(name) == "" {
		return fmt.Errorf("give the ruin a name")
	}
	for path, obj := range c.Dme.Objects {
		if path == except || !strings.HasPrefix(path, Type+"/") || text(obj.Vars, "suffix") == "" {
			continue
		}
		current := text(obj.Vars, "name")
		if normalized(current) == normalized(name) {
			return fmt.Errorf("a ruin named %q already exists", current)
		}
	}
	return nil
}
func (c *Catalog) SuggestID(name string, loc Location) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if separator && b.Len() > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r)
			separator = false
		} else {
			separator = true
		}
	}
	base := b.String()
	if base == "" {
		base = "ruin"
	}
	if base[0] < 'a' || base[0] > 'z' {
		base = "ruin_" + base
	}
	if len(base) > 38 {
		base = strings.TrimRight(base[:38], "_")
	}
	id := base
	// If the directory is inaccessible, validation can explain the problem
	// without an unbounded search blocking the UI.
	for n := 2; n <= 1000 && c.idUsed(id, loc); n++ {
		id = base + "_" + strconv.Itoa(n)
	}
	return id
}
func (c *Catalog) idUsed(id string, loc Location) bool {
	if c.Dme.Objects[loc.Type+"/"+id] != nil || c.Dme.Objects["/area/ruin/"+id] != nil {
		return true
	}
	for _, t := range c.Templates {
		if strings.EqualFold(t.ID, id) {
			return true
		}
	}
	for _, rel := range []string{loc.Prefix + id + ".dmm", c.sourceRelative(loc.Type + "/" + id)} {
		file, err := ship.Inside(c.Dme.RootDir, rel)
		if err != nil || exists(file) {
			return true
		}
	}
	return false
}
func (c *Catalog) sourceRelative(path string) string {
	root := "code/datums/ruins/workshop"
	if c.Dme.Objects[ship.HullType] != nil {
		root = "voidcrew/mapping/ruins"
	}
	return filepath.ToSlash(filepath.Join(root, strings.TrimPrefix(path, Type+"/")+".dm"))
}

type Properties struct {
	Name, Description                        string
	Cost, MineralCost, Weight                float64
	AllowDuplicates, AlwaysPlace, Unpickable bool
}

func ReadProperties(obj *dmenv.Object) (Properties, error) {
	p := Properties{Name: text(obj.Vars, "name"), Description: text(obj.Vars, "description")}
	for _, f := range []struct {
		key      string
		target   *float64
		fallback string
	}{
		{"cost", &p.Cost, "0"}, {"mineral_cost", &p.MineralCost, "0"}, {"placement_weight", &p.Weight, "1"},
	} {
		n, err := strconv.ParseFloat(obj.Vars.ValueV(f.key, f.fallback), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return p, fmt.Errorf("%s uses a custom %s expression; open its map to edit the layout", obj.Path, f.key)
		}
		*f.target = n
	}
	for _, f := range []struct {
		key      string
		target   *bool
		fallback string
	}{
		{"allow_duplicates", &p.AllowDuplicates, "1"}, {"always_place", &p.AlwaysPlace, "0"}, {"unpickable", &p.Unpickable, "0"},
	} {
		switch obj.Vars.ValueV(f.key, f.fallback) {
		case "TRUE", "1":
			*f.target = true
		case "FALSE", "0", "null":
			*f.target = false
		default:
			return p, fmt.Errorf("%s has a custom %s expression", obj.Path, f.key)
		}
	}
	return p, nil
}
func (p Properties) Validate() error {
	if normalized(p.Name) == "" {
		return fmt.Errorf("give the ruin a name")
	}
	for _, n := range []float64{p.Cost, p.MineralCost, p.Weight} {
		if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			return fmt.Errorf("budgets and spawn weight must be finite, non-negative numbers")
		}
	}
	return nil
}
func (p Properties) values() map[string]string {
	boolDM := func(b bool) string {
		if b {
			return "TRUE"
		}
		return "FALSE"
	}
	return map[string]string{"name": quote(p.Name), "description": quote(p.Description), "cost": strconv.FormatFloat(p.Cost, 'f', -1, 64), "mineral_cost": strconv.FormatFloat(p.MineralCost, 'f', -1, 64), "placement_weight": strconv.FormatFloat(p.Weight, 'f', -1, 64), "allow_duplicates": boolDM(p.AllowDuplicates), "always_place": boolDM(p.AlwaysPlace), "unpickable": boolDM(p.Unpickable)}
}
