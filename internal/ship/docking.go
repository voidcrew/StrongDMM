package ship

import (
	"fmt"
	"strconv"
	"strings"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// Directions are clockwise, matching BYOND's dir2angle arithmetic.
var compass = []int{1, 4, 2, 8}

func directionIndex(dir int) int {
	for i, value := range compass {
		if value == dir {
			return i
		}
	}
	return -1
}

func portDirection(port *dmmprefab.Prefab, name string, fallback int) int {
	value := port.Vars().ValueV(name, strconv.Itoa(fallback))
	for i, word := range []string{"NORTH", "EAST", "SOUTH", "WEST"} {
		if value == word {
			return compass[i]
		}
	}
	dir, _ := strconv.Atoi(value)
	return dir
}

type DockingSite struct {
	Position       util.Point
	Directions     []int // Outward directions with no hull extending beyond the door.
	CurrentOutward int
	Existing       bool
	Airlock        bool
	port           *dmmprefab.Prefab
	hull           *dmmap.Dmm
}

func childPath(path, root string) bool { return path == root || strings.HasPrefix(path, root+"/") }

// InspectDockingSite accepts one tile, or a Grab selection containing one airlock.
// The hull owns the port even when the selection originated in a module pane.
func (p *Project) InspectDockingSite(theme Theme, lo, hi util.Point) (*DockingSite, error) {
	file, err := p.Catalog.HullFile(p.Hull, theme)
	if err != nil {
		return nil, err
	}
	doc, err := p.document(file)
	if err != nil {
		return nil, err
	}
	m := doc.Map
	if lo.Z != 1 || hi.Z != 1 || lo.X > hi.X || lo.Y > hi.Y || !m.HasTile(lo) || !m.HasTile(hi) {
		return nil, fmt.Errorf("use Grab (3) to select an airlock in the hull")
	}
	site := &DockingSite{Position: lo, hull: m}
	doors, ports := 0, 0
	for _, tile := range m.Tiles {
		for _, inst := range tile.Instances() {
			prefab := inst.Prefab()
			if childPath(prefab.Path(), "/obj/docking_port/mobile") {
				ports++
				site.port = prefab
			}
			if childPath(prefab.Path(), "/obj/machinery/door/airlock") && tile.Coord.Z == lo.Z && tile.Coord.X >= lo.X && tile.Coord.X <= hi.X && tile.Coord.Y >= lo.Y && tile.Coord.Y <= hi.Y {
				doors++
				site.Position = tile.Coord
			}
		}
	}
	if doors > 1 || (lo != hi && doors != 1) {
		return nil, fmt.Errorf("select just one airlock, or a single tile for the docking entrance")
	}
	site.Airlock = doors == 1
	if ports > 1 {
		return nil, fmt.Errorf("the hull has multiple mobile ports; remove the extra ports first")
	}
	site.Existing = ports == 1
	if !site.Existing {
		if p.Settings == nil {
			return nil, fmt.Errorf("this hull has no mobile port; place its ship-specific port type first")
		}
		site.port = p.prefab(p.portType(), map[string]string{"dir": "1"})
	}
	root := site.port.Vars().ValueV("area_type", "")
	if !strings.HasPrefix(root, "/area/shuttle/voidcrew/") || p.Dme.Objects[root] == nil {
		return nil, fmt.Errorf("the mobile port needs a ship-specific area type")
	}
	owned := func(tile *dmmap.Tile) bool {
		for _, inst := range tile.Instances() {
			if childPath(inst.Prefab().Path(), root) {
				return true
			}
		}
		return false
	}
	target := m.GetTile(site.Position)
	if !owned(target) {
		return nil, fmt.Errorf("assign a ship area to the entrance tile first")
	}
	floor := false
	for _, inst := range target.Instances() {
		path := inst.Prefab().Path()
		if childPath(path, "/turf/open") && !childPath(path, "/turf/open/space") {
			floor = true
		}
	}
	if !floor {
		return nil, fmt.Errorf("the docking entrance needs an open floor tile")
	}
	for _, dir := range compass {
		edge := true
		for _, tile := range m.Tiles {
			if tile.Coord.Z != site.Position.Z || !owned(tile) {
				continue
			}
			at, seat := tile.Coord, site.Position
			if dir == 1 && at.Y > seat.Y || dir == 2 && at.Y < seat.Y || dir == 4 && at.X > seat.X || dir == 8 && at.X < seat.X {
				edge = false
				break
			}
		}
		if edge {
			site.Directions = append(site.Directions, dir)
		}
	}
	if len(site.Directions) == 0 {
		return nil, fmt.Errorf("choose an entrance on an outside edge; the hull extends past this tile")
	}
	inward := directionIndex(portDirection(site.port, "dir", 1))
	relative := directionIndex(portDirection(site.port, "port_direction", 1))
	if inward < 0 || relative < 0 {
		return nil, fmt.Errorf("the existing port must use cardinal directions before it can be moved")
	}
	site.CurrentOutward = compass[(inward+2)%4]
	return site, nil
}

// PlaceDockingPort revalidates the selection, moves (or creates) one port, and
// preserves its subtype and other overrides. Runtime computes mobile bounds.
func (p *Project) PlaceDockingPort(theme Theme, lo, hi util.Point, outward int) error {
	site, err := p.InspectDockingSite(theme, lo, hi)
	if err != nil {
		return err
	}
	allowed := false
	for _, dir := range site.Directions {
		allowed = allowed || dir == outward
	}
	if !allowed {
		return fmt.Errorf("choose an outward direction with no hull beyond the entrance")
	}
	inward := (directionIndex(outward) + 2) % 4
	oldInward := directionIndex(portDirection(site.port, "dir", 1))
	oldRelative := directionIndex(portDirection(site.port, "port_direction", 1))
	// Same rotation as hull_port_facing(): rotate the relative direction by
	// the port's turn, without changing which way the ship itself faces.
	relative := (oldRelative + inward - oldInward + 4) % 4
	w, h := site.hull.MaxX, site.hull.MaxY
	if inward%2 == 1 {
		w, h = h, w
	}
	if relative%2 == 1 {
		w, h = h, w
	}
	preferred := 1
	if h > w {
		preferred = 4 // Matches adjust_reserve_dock_to_shuttle / hull_aspect_guess.
	}
	vars := dmvars.Set(site.port.Vars(), "dir", strconv.Itoa(compass[inward]))
	vars = dmvars.Set(vars, "port_direction", strconv.Itoa(compass[relative]))
	vars = dmvars.Set(vars, "preferred_direction", strconv.Itoa(preferred))
	port := dmmap.PrefabStorage.Put(dmmprefab.New(0, site.port.Path(), vars))
	for _, tile := range site.hull.Tiles {
		prefabs := dmmdata.Prefabs{}
		changed := tile.Coord == site.Position
		for _, inst := range tile.Instances() {
			if childPath(inst.Prefab().Path(), "/obj/docking_port/mobile") {
				changed = true
			} else {
				prefabs = append(prefabs, inst.Prefab())
			}
		}
		if tile.Coord == site.Position {
			prefabs = append(dmmdata.Prefabs{port}, prefabs...)
		}
		if changed {
			tile.InstancesSet(prefabs)
		}
	}
	return nil
}
