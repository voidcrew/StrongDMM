package planet

import (
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
)

// RiverTurf mirrors spawn_planet_rivers_for in Voidcrew. Rivers belong to the
// overmap environment, not a biome table; new planets inherit that environment.
func RiverTurf(c *Catalog, s State) string {
	if r := s.Definition.Rivers; r != nil {
		if r.Enabled {
			return r.Turf
		}
		return ""
	}
	o := c.Dme.Objects[s.Definition.Overmap]
	if o == nil {
		o = c.Dme.Objects[s.BaseOvermap]
	}
	if o == nil {
		return ""
	}
	switch o.Vars.TextV("ruin_type", "") {
	case "Lava Ruins":
		return "/turf/open/lava/smooth/lava_land_surface/planetary"
	case "Ice Ruins":
		return "/turf/open/lava/plasma/planetary"
	}
	return ""
}

type riverPoint struct{ x, y int }

// Reproduce the four-node walk and SpreadAcrossPlanet rules with an independent
// seeded stream. Game rivers use BYOND's shared random stream, so the preview is
// a repeatable study, not a prediction of a particular live round.
func (p *Preview) rivers(s State, dme *dmenv.Dme, path string, ox, oy int, prefab func(string) *dmmprefab.Prefab) {
	settings := s.Definition.Rivers
	if settings == nil {
		settings = defaultRivers()
	}
	size := p.Map.MaxX
	counter := 0
	random := func() float64 {
		counter++
		return roll(s.Seeds.Detail^0x72697672, ox, oy, counter)
	}
	inside := func(q riverPoint) bool { return q.x >= 0 && q.y >= 0 && q.x < size && q.y < size }
	cell := func(q riverPoint) *Cell { return &p.Cells[q.y*size+q.x] }
	allowed := func(q riverPoint) bool {
		if !inside(q) {
			return false
		}
		o := dme.Objects[cell(q).Turf]
		if o != nil && o.Vars.IntV("turf_flags", 0)&8 != 0 { // NO_LAVA_GEN
			return false
		}
		if settings.Biomes != nil {
			for _, path := range settings.Biomes {
				if cell(q).Biome == path {
					return true
				}
			}
			return false
		}
		return true
	}
	place := func(q riverPoint, turf string) {
		c := cell(q)
		c.Turf, c.Closed, c.River = turf, dm.IsPath(turf, "/turf/closed"), turf == path
		tile := p.Map.Tiles[q.y*size+q.x]
		for _, instance := range tile.Instances() {
			if dm.IsPath(instance.Prefab().Path(), "/turf") {
				instance.SetPrefab(prefab(turf))
				break
			}
		}
	}
	floor := func(q riverPoint) string {
		if o := dme.Objects[cell(q).Turf]; o != nil && dm.IsPath(cell(q).Turf, "/turf/closed/mineral") {
			return o.Vars.ValueV("turf_type", "")
		}
		return ""
	}
	var spread func(riverPoint, float64)
	spread = func(q riverPoint, probability float64) {
		if probability <= 0 {
			return
		}
		var cardinals, diagonals []riverPoint
		loggedFloor := ""
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				n := riverPoint{q.x + dx, q.y + dy}
				if !allowed(n) {
					continue
				}
				turf := cell(n).Turf
				mineral := dm.IsPath(turf, "/turf/closed/mineral")
				dense := cell(n).Closed
				if o := dme.Objects[turf]; o != nil {
					dense = o.Vars.IntV("density", 0) != 0
				}
				if (dense && !mineral) || dm.IsPath(turf, "/turf/open/indestructible") {
					continue
				}
				if loggedFloor == "" && mineral {
					loggedFloor = floor(n)
				}
				if dx == 0 || dy == 0 {
					cardinals = append(cardinals, n)
				} else {
					diagonals = append(diagonals, n)
				}
			}
		}
		for _, n := range cardinals {
			if loggedFloor != "" && dm.IsPath(cell(n).Turf, loggedFloor) {
				continue
			}
			place(n, path)
			if random()*100 < float64(probability) {
				spread(n, probability-settings.Loss)
			}
		}
		for _, n := range diagonals {
			if (loggedFloor == "" || !dm.IsPath(cell(n).Turf, loggedFloor)) && random()*100 < float64(probability) {
				place(n, path)
				spread(n, probability-settings.Loss)
			} else if f := floor(n); dm.IsPath(f, "/turf/") && dme.Objects[f] != nil {
				place(n, f)
			}
		}
	}
	var locations []riverPoint
	for y := 0; y < size-1; y++ {
		for x := 0; x < size-1; x++ {
			q := riverPoint{x, y}
			if allowed(q) {
				locations = append(locations, q)
			}
		}
	}
	if len(locations) == 0 {
		return
	}
	nodes := make([]riverPoint, settings.Nodes)
	for i := range nodes {
		nodes[i] = locations[int(random()*float64(len(locations)))]
	}
	directions := []riverPoint{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}
	toward := func(a, b riverPoint) int {
		dx, dy := max(-1, min(1, b.x-a.x)), max(-1, min(1, b.y-a.y))
		for i, d := range directions {
			if d.x == dx && d.y == dy {
				return i
			}
		}
		return 0
	}
	step := func(q riverPoint, dir int) riverPoint {
		d := directions[dir]
		return riverPoint{q.x + d.x, q.y + d.y}
	}
	for i, q := range nodes {
		place(q, path)
		target := nodes[(i+1+int(random()*float64(len(nodes)-1)))%len(nodes)]
		dir, detouring := toward(q, target), false
		for steps := 0; q != target && steps < size*size; steps++ {
			if detouring {
				if random() < .2 {
					detouring, dir = false, toward(q, target)
				}
			} else if random()*100 < settings.Detour {
				detouring = true
				turn := 1
				if random() < .5 {
					turn = 7
				}
				dir = (dir + turn) % len(directions)
			} else {
				dir = toward(q, target)
			}
			next := step(q, dir)
			if !inside(next) {
				detouring, dir = false, toward(q, target)
				next = step(q, dir)
			}
			q = next
			if !allowed(q) {
				detouring, dir = false, toward(q, target)
				if q != target {
					q = step(q, dir)
				}
				continue
			}
			place(q, path)
			spread(q, settings.Spread)
		}
	}
}
