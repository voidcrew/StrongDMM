package mappreview

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

type smoothingRules struct{ bitmask, cardinals, diagonal, border, borderObject, procFilter int }

// tg changed the bit assignments when the older corner system was retired.
// Read the project's defines rather than baking one revision's flags into maps.
func loadRules(root string) smoothingRules {
	r := smoothingRules{1, 2, 4, 8, 64, 128}
	data, err := os.ReadFile(filepath.Join(root, "code/__DEFINES/icon_smoothing.dm"))
	if err != nil {
		return r
	}
	defines := regexp.MustCompile(`(?m)^\s*#define\s+(SMOOTH_\w+)\s+\(?\s*(\d+)\s*(?:<<\s*(\d+))?\s*\)?`).FindAllStringSubmatch(string(data), -1)
	values := map[string]int{}
	for _, match := range defines {
		value, _ := strconv.Atoi(match[2])
		if match[3] != "" {
			shift, _ := strconv.Atoi(match[3])
			value <<= shift
		}
		values[match[1]] = value
	}
	if value, ok := values["SMOOTH_BITMASK"]; ok {
		r = smoothingRules{bitmask: value, cardinals: values["SMOOTH_BITMASK_CARDINALS"], diagonal: values["SMOOTH_DIAGONAL_CORNERS"], border: values["SMOOTH_BORDER"], borderObject: values["SMOOTH_BORDER_OBJECT"], procFilter: values["SMOOTH_PROC_FILTER"]}
		if r.diagonal == 0 {
			r.diagonal = values["SMOOTH_DIAGONAL"]
		}
	}
	return r
}

var groupToken = regexp.MustCompile(`"[^"]*"|[-+]?\d+`)

// Both current comma-separated group strings and older list(...) declarations
// arrive here with macros already evaluated by the DM environment parser.
func groups(raw string) map[string]bool {
	result := map[string]bool{}
	for _, match := range groupToken.FindAllString(raw, -1) {
		for _, value := range strings.Split(strings.Trim(match, `"`), ",") {
			value = strings.TrimSpace(value)
			if value != "" {
				result[value] = true
			}
		}
	}
	return result
}

func smoothJunction(source *dmmap.Dmm, coord util.Point, prefab *dmmprefab.Prefab, r smoothingRules) int {
	vars := prefab.Vars()
	wanted := groups(vars.ValueV("canSmoothWith", "null"))
	flags := vars.IntV("smoothing_flags", 0)
	areaLimit := ""
	for _, item := range source.GetTile(coord).Instances() {
		if dm.IsPath(item.Prefab().Path(), "/area") {
			areaLimit = item.Prefab().Vars().ValueV("area_limited_icon_smoothing", "null")
		}
	}
	joins := func(dx, dy int) bool {
		next := util.Point{X: coord.X + dx, Y: coord.Y + dy, Z: coord.Z}
		if !source.HasTile(next) {
			return flags&r.border != 0
		}
		tile := source.GetTile(next)
		if strings.HasPrefix(areaLimit, "/area") {
			allowed := false
			for _, item := range tile.Instances() {
				if dm.IsPath(item.Prefab().Path(), areaLimit) {
					allowed = true
				}
			}
			if !allowed {
				return flags&r.border != 0
			}
		}
		for _, item := range tile.Instances() {
			other := item.Prefab()
			if dm.IsPath(other.Path(), "/area") {
				continue
			}
			if !dm.IsPath(other.Path(), "/turf") && other.Vars().IntV("anchored", 0) == 0 {
				continue
			}
			if len(wanted) == 0 {
				if other.Path() == prefab.Path() {
					return true
				}
				continue
			}
			for group := range groups(other.Vars().ValueV("smoothing_groups", "null")) {
				if wanted[group] {
					return true
				}
			}
		}
		return false
	}
	junction := 0
	for _, direction := range [][3]int{{0, 1, 1}, {0, -1, 2}, {1, 0, 4}, {-1, 0, 8}} {
		if joins(direction[0], direction[1]) {
			junction |= direction[2]
		}
	}
	if flags&r.cardinals == 0 {
		for _, direction := range [][4]int{{1, 1, 5, 16}, {1, -1, 6, 32}, {-1, -1, 10, 64}, {-1, 1, 9, 128}} {
			if junction&direction[2] == direction[2] && joins(direction[0], direction[1]) {
				junction |= direction[3]
			}
		}
	}
	return junction
}

func diagonalJunction(j int) bool {
	switch j {
	case 5, 6, 9, 10, 21, 38, 74, 137:
		return true
	}
	return false
}

var underlayField = regexp.MustCompile(`"(icon|icon_state|dir|space)"\s*=\s*('[^']*'|"[^"]*"|[-\d]+)`)

func diagonalUnderlay(source *dmmap.Dmm, dme *dmenv.Dme, coord util.Point, wall *dmmprefab.Prefab, junction int) *dmmprefab.Prefab {
	fields := map[string]string{}
	for _, match := range underlayField.FindAllStringSubmatch(wall.Vars().ValueV("fixed_underlay", "null"), -1) {
		fields[match[1]] = match[2]
	}
	var base *dmmprefab.Prefab
	if fields["space"] == "1" {
		if object := dme.Objects["/turf/open/space"]; object != nil {
			base = dmmprefab.New(1, object.Path, object.Vars)
		}
	} else if len(fields) == 0 {
		// Copy an open turf from the exposed side of the diagonal corner.
		dx, dy := 1, 1
		if junction&4 != 0 {
			dx = -1
		}
		if junction&1 != 0 {
			dy = -1
		}
		for _, offset := range [][2]int{{0, dy}, {dx, 0}, {dx, dy}} {
			at := util.Point{X: coord.X + offset[0], Y: coord.Y + offset[1], Z: coord.Z}
			if !source.HasTile(at) {
				continue
			}
			for _, item := range source.GetTile(at).Instances() {
				if dm.IsPath(item.Prefab().Path(), "/turf/open") {
					base = item.Prefab()
					break
				}
			}
			if base != nil {
				break
			}
		}
	}
	if base == nil {
		if object := dme.Objects["/turf/open/floor/plating"]; object != nil {
			base = dmmprefab.New(1, object.Path, object.Vars)
		}
	}
	if base == nil {
		return nil
	}
	delete(fields, "space")
	// Use the floor's own plane/layer even when fixed_underlay changes its sprite.
	vars := dmvars.FromParent(base.Vars())
	for name, value := range fields {
		vars = dmvars.Set(vars, name, value)
	}
	return dmmprefab.New(1, base.Path(), vars)
}
