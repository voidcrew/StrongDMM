package planet

import (
	"fmt"
	"math"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
	"sdmm/third_party/sdmmparser"
)

type PreviewOptions struct {
	Biome            string
	Caves, Populate  bool
	OriginX, OriginY int
}
type Cell struct {
	Biome, Turf            string
	Spawn, SpawnField      string
	Heat, Moisture, Height float64
	Closed, River          bool
}
type Preview struct {
	Map    *dmmap.Dmm
	Cells  []Cell
	Counts map[string]int
}

// Stateless rolls keep unrelated tiles and tables steady during live editing.
func roll(seed uint32, x, y, channel int) float64 {
	n := uint64(seed) ^ uint64(x)*0x9e3779b97f4a7c15 ^ uint64(y)*0xbf58476d1ce4e5b9 ^ uint64(channel)*0x94d049bb133111eb
	n = (n ^ (n >> 30)) * 0xbf58476d1ce4e5b9
	n = (n ^ (n >> 27)) * 0x94d049bb133111eb
	n ^= n >> 31
	return float64(n>>11) / float64(uint64(1)<<53)
}
func pick(entries []Entry, r float64) string {
	total := 0.0
	for _, e := range entries {
		total += e.Weight
	}
	r *= total
	for _, e := range entries {
		r -= e.Weight
		if r < 0 {
			return e.Path
		}
	}
	if len(entries) > 0 {
		return entries[len(entries)-1].Path
	}
	return ""
}

type field struct {
	values []float64
	width  int
	x, y   float64
}

func noiseField(seed uint32, size, originX, originY int, zoom float64) field {
	w := (size+3)/4 + 2
	x, y := float64(originX-2), float64(originY-2)
	return field{sdmmparser.PlanetNoise(seed, x/zoom, y/zoom, 4/zoom, w, w), w, x, y}
}
func (f field) sample(x, y float64) float64 {
	x = (x - f.x) / 4
	y = (y - f.y) / 4
	gx, gy := int(math.Floor(x)), int(math.Floor(y))
	tx, ty := x-float64(gx), y-float64(gy)
	i := gy*f.width + gx
	a := f.values[i] + (f.values[i+1]-f.values[i])*tx
	b := f.values[i+f.width] + (f.values[i+f.width+1]-f.values[i+f.width])*tx
	return math.Max(0, math.Min(1, a+(b-a)*ty))
}
func band(value float64, ends ...float64) int {
	for i, end := range ends {
		if value <= end {
			return i
		}
	}
	return len(ends)
}

// rust-g's automaton has a clear guard border and one extra simulated cell.
// Its returned string is x-major; DM indexes that string as world-width rows.
// Preserve both details, with a seeded initial fill for reproducible editing.
func caveMask(seed uint32, g Generator, width, height int) []bool {
	w, h := width+3, height+3
	grid := make([]bool, w*h)
	for x := 1; x < w-1; x++ {
		for y := 1; y < h-1; y++ {
			grid[x*h+y] = roll(seed, x, y, 11) < g.Closed/100
		}
	}
	for i := 0; i < g.Iterations; i++ {
		next := make([]bool, len(grid))
		for x := 1; x < w-1; x++ {
			for y := 1; y < h-1; y++ {
				n := 0
				for dx := -1; dx <= 1; dx++ {
					for dy := -1; dy <= 1; dy++ {
						if (dx != 0 || dy != 0) && grid[(x+dx)*h+y+dy] {
							n++
						}
					}
				}
				old := grid[x*h+y]
				next[x*h+y] = (old && n >= g.Death) || (!old && n > g.Birth)
			}
		}
		grid = next
	}
	out := make([]bool, width*height)
	i := 0
	for x := 1; x <= width; x++ {
		for y := 1; y <= height; y++ {
			out[i] = grid[x*h+y]
			i++
		}
	}
	return out
}

func Generate(c *Catalog, s State, dme *dmenv.Dme, o PreviewOptions) (*Preview, error) {
	if err := s.Validate(c); err != nil {
		return nil, err
	}
	size := s.Size
	if o.Biome != "" {
		size = 48
		if _, ok := s.Biomes[o.Biome]; !ok {
			return nil, fmt.Errorf("Select a biome to preview.")
		}
	}
	ox, oy := o.OriginX, o.OriginY
	if ox == 0 {
		ox = 3
	}
	if oy == 0 {
		oy = 3
	}
	heat := noiseField(s.Seeds.Heat, size, ox, oy, s.Definition.Settings.Zoom)
	wet := noiseField(s.Seeds.Moisture, size, ox, oy, s.Definition.Settings.Zoom)
	high := noiseField(s.Seeds.Height, size, ox, oy, s.Definition.Settings.Zoom)
	worldSize := max(255, ox+size, oy+size)
	caves := caveMask(s.Seeds.Detail, s.Definition.Settings, worldSize, worldSize)
	p := &Preview{Map: &dmmap.Dmm{Name: s.Definition.Name, MaxX: size, MaxY: size, MaxZ: 1}, Counts: map[string]int{}}
	p.Cells = make([]Cell, size*size)
	p.Map.Tiles = make([]*dmmap.Tile, size*size)
	prefabs := map[string]*dmmprefab.Prefab{}
	prefab := func(path string) *dmmprefab.Prefab {
		if f := prefabs[path]; f != nil {
			return f
		}
		v := &dmvars.Variables{}
		if obj := dme.Objects[path]; obj != nil {
			v = dmvars.FromParent(obj.Vars)
		}
		if e := s.Definition.Environment; e != nil {
			if path == s.Definition.Area || path == s.BaseArea {
				v = dmvars.Set(v, "static_lighting", "0")
				v = dmvars.Set(v, "ambient_lighting", "1")
				v = dmvars.Set(v, "base_lighting_color", quote(e.LightColor))
				v = dmvars.Set(v, "base_lighting_alpha", number(e.LightAlpha))
			}
		}
		f := dmmprefab.New(0, path, v)
		prefabs[path] = f
		return f
	}
	area := s.Definition.Area
	if dme.Objects[area] == nil {
		area = s.BaseArea
	}
	if dme.Objects[area] == nil {
		area = "/area"
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			gx, gy := x+ox, y+oy
			dx := float64(gx + int(roll(s.Seeds.Detail, gx, gy, 0)*5) - 2)
			dy := float64(gy + int(roll(s.Seeds.Detail, gx, gy, 1)*5) - 2)
			cell := Cell{Heat: heat.sample(dx, dy), Moisture: wet.sample(dx, dy), Height: high.sample(float64(gx), float64(gy))}
			isCave := len(s.Definition.Caves) > 0 && (o.Caves || len(s.Definition.Surface) == 0 || cell.Height > s.Definition.Settings.Mountain)
			if o.Biome != "" {
				cell.Biome = o.Biome
				isCave = s.Biomes[o.Biome].Cave
			} else {
				grid := s.Definition.Surface
				row := band(cell.Heat, .2, .4, .6, .65, .8)
				if isCave {
					grid = s.Definition.Caves
					row = band(cell.Heat, .25, .5, .75)
				}
				if len(grid) == 0 {
					return nil, fmt.Errorf("This planet has no biomes for this layer.")
				}
				cell.Biome = grid[row][band(cell.Moisture, .2, .4, .6, .8)]
			}
			b := s.Biomes[cell.Biome]
			field := "open_turf_types"
			cell.Closed = isCave && caves[worldSize*(gy-1)+gx-1]
			if cell.Closed {
				field = "closed_turf_types"
			}
			for _, t := range b.Tables {
				if t.Field == field {
					cell.Turf = pick(t.Entries, roll(s.Seeds.Detail, gx, gy, 2))
					break
				}
			}
			if cell.Turf == "" {
				return nil, fmt.Errorf("%s has no %s.", b.Name, field)
			}
			i := y*size + x
			p.Cells[i] = cell
			p.Counts[cell.Biome]++
			a := area
			if isCave && dme.Objects["/area/overmap_encounter/planetoid/cave"] != nil {
				a = "/area/overmap_encounter/planetoid/cave"
			}
			tile := &dmmap.Tile{Coord: util.Point{X: x + 1, Y: y + 1, Z: 1}}
			tile.InstancesSet(dmmdata.Prefabs{prefab(cell.Turf), prefab(a)})
			p.Map.Tiles[i] = tile
		}
	}
	if river := RiverTurf(c, s); river != "" && dme.Objects[river] != nil && !o.Caves && o.Biome == "" {
		p.rivers(s, dme, river, ox, oy, prefab)
	}
	if o.Populate {
		type spawn struct {
			x, y int
			path string
		}
		var features, mobs []spawn
		near := func(list []spawn, x, y, radius int, path string) bool {
			for _, a := range list {
				if (path == "" || a.path == path) && abs(x-a.x) <= radius && abs(y-a.y) <= radius {
					return true
				}
			}
			return false
		}
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				i := y*size + x
				cell := p.Cells[i]
				if cell.Closed || cell.River {
					continue
				}
				b := s.Biomes[cell.Biome]
				eligible := false
				for _, table := range b.Tables {
					if table.Field == "open_turf_types" {
						for _, entry := range table.Entries {
							eligible = eligible || cell.Turf == entry.Path
						}
					}
				}
				if !eligible {
					continue
				}
				for j, field := range []string{"flora_spawn_list", "feature_spawn_list", "mob_spawn_list"} {
					var table Table
					for _, t := range b.Tables {
						if t.Field == field {
							table = t
							break
						}
					}
					if len(table.Entries) == 0 || roll(s.Seeds.Detail, x+ox, y+oy, 20+j*2)*100 >= table.Chance {
						continue
					}
					path := pick(table.Entries, roll(s.Seeds.Detail, x+ox, y+oy, 21+j*2))
					for attempt := 0; path == MegafaunaRoll && attempt < 10; attempt++ {
						path = pick(table.Entries, roll(s.Seeds.Detail, x+ox, y+oy, 40+attempt))
					}
					if path == MegafaunaRoll {
						continue
					}
					if j == 1 && near(features, x, y, 7, path) {
						continue
					}
					if j == 2 && near(mobs, x, y, 12, "") {
						continue
					}
					p.Map.Tiles[i].InstancesAdd(prefab(path))
					// A type can occur in multiple tables; retain the actual roll.
					p.Cells[i].Spawn, p.Cells[i].SpawnField = path, field
					if j == 1 {
						features = append(features, spawn{x, y, path})
					}
					if j == 2 {
						mobs = append(mobs, spawn{x, y, path})
					}
					break
				}
			}
		}
	}
	return p, nil
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
