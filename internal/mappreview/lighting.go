package mappreview

import (
	"math"
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/util"
)

type RGB [3]float32

// Lighting stores tg-style shared corner colors. Ambient area lighting is kept
// per tile so a fullbright area does not illuminate an adjacent dark room.
type Lighting struct {
	Width, Height int
	Corners       []RGB
	Ambient       []RGB
	Fullbright    []bool
}

func (l *Lighting) Tile(x, y int) [4]RGB {
	index := y*l.Width + x
	if l.Fullbright[index] {
		return [4]RGB{{1, 1, 1}, {1, 1, 1}, {1, 1, 1}, {1, 1, 1}}
	}
	stride := l.Width + 1
	result := [4]RGB{l.Corners[y*stride+x], l.Corners[y*stride+x+1], l.Corners[(y+1)*stride+x], l.Corners[(y+1)*stride+x+1]}
	for i := range result {
		for channel := range result[i] {
			result[i][channel] = min(1, result[i][channel]+l.Ambient[index][channel])
		}
	}
	return result
}

type lightSource struct {
	x, y, radius, power, angle, height float64
	dir                                int
	color                              RGB
	tileX, tileY                       int
}

func colorRGB(text string) RGB {
	if text == "" {
		return RGB{1, 1, 1}
	}
	r, g, b, _ := util.ParseColor(text).RGBA()
	return RGB{r, g, b}
}

func mappedLight(prefab *dmmprefab.Prefab, x, y, iconSize int, powered bool) (lightSource, bool) {
	v := prefab.Vars()
	light := lightSource{x: float64(x) + .5, y: float64(y) + .5, tileX: x, tileY: y,
		radius: float64(v.FloatV("light_range", 0)), power: float64(v.FloatV("light_power", 1)),
		angle: float64(v.FloatV("light_angle", 360)), height: float64(v.FloatV("light_height", 1)),
		dir: v.IntV("light_dir", 1), color: colorRGB(v.TextV("light_color", "#ffffff"))}
	if dm.IsPath(prefab.Path(), "/obj/machinery/light") {
		if v.IntV("status", 0) != 0 || (!powered && v.IntV("on", 0) == 0) {
			return light, false
		}
		light.radius = float64(v.FloatV("brightness", 8))
		light.power = float64(v.FloatV("bulb_power", 1))
		light.color = colorRGB(v.TextV("bulb_colour", v.TextV("bulb_color", "#ffffff")))
		dir := v.IntV("dir", 2)
		light.dir = ((dir & 1) << 1) | ((dir & 2) >> 1) | ((dir & 4) << 1) | ((dir & 8) >> 1)
	} else if v.IntV("light_on", 1) == 0 {
		return light, false
	}
	if light.radius <= 0 || light.power == 0 {
		return light, false
	}
	light.radius = max(1.4, light.radius)
	// tg positions light independently of map-grid coordinates using appearance offsets.
	if v.IntV("light_flags", 0)&4 == 0 {
		light.x += float64(v.IntV("pixel_x", 0)+v.IntV("pixel_w", 0)) / float64(iconSize)
		light.y += float64(v.IntV("pixel_y", 0)+v.IntV("pixel_z", 0)) / float64(iconSize)
	}
	return light, true
}

func buildLighting(source *dmmap.Dmm, powered, exterior bool) (*Lighting, int) {
	w, h := source.MaxX, source.MaxY
	lightmap := &Lighting{Width: w, Height: h, Corners: make([]RGB, (w+1)*(h+1)), Ambient: make([]RGB, w*h), Fullbright: make([]bool, w*h)}
	opaque := make([]bool, w*h)
	var sources []lightSource
	iconSize := dmmap.WorldIconSize
	if iconSize <= 0 {
		iconSize = 32
	}
	for _, tile := range source.Tiles {
		x, y := tile.Coord.X-1, tile.Coord.Y-1
		index := y*w + x
		for _, instance := range tile.Instances() {
			prefab := instance.Prefab()
			v := prefab.Vars()
			if dm.IsPath(prefab.Path(), "/area") {
				lightmap.Fullbright[index] = v.IntV("static_lighting", v.IntV("dynamic_lighting", 1)) == 0
				color := colorRGB(v.TextV("base_lighting_color", "#ffffff"))
				alpha := v.FloatV("base_lighting_alpha", 0) / 255
				for c := range color {
					lightmap.Ambient[index][c] = color[c] * alpha
				}
				continue
			}
			if v.IntV("opacity", 0) != 0 {
				opaque[index] = true
			}
			if light, ok := mappedLight(prefab, x, y, iconSize, powered); ok {
				sources = append(sources, light)
			}
		}
	}
	if exterior {
		illuminateExterior(source, lightmap, opaque)
	}
	visited := make([]int, len(lightmap.Corners))
	for number, light := range sources {
		radius := int(math.Ceil(light.radius + math.Abs(light.x-float64(light.tileX)-.5) + math.Abs(light.y-float64(light.tileY)-.5)))
		for y := max(0, light.tileY-radius); y <= min(h-1, light.tileY+radius); y++ {
			for x := max(0, light.tileX-radius); x <= min(w-1, light.tileX+radius); x++ {
				// tg adds corners from visible non-opaque turfs, then shares them
				// with the wall face bordering those turfs.
				if opaque[y*w+x] || !lineVisible(opaque, w, h, light.tileX, light.tileY, x, y) {
					continue
				}
				for _, offset := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
					cx, cy := x+offset[0], y+offset[1]
					index := cy*(w+1) + cx
					if visited[index] == number+1 {
						continue
					}
					visited[index] = number + 1
					strength := float32(light.falloff(float64(cx)-light.x, float64(cy)-light.y) * light.power)
					for c := 0; c < 3; c++ {
						lightmap.Corners[index][c] += light.color[c] * strength
					}
				}
			}
		}
	}
	for index, color := range lightmap.Corners {
		scale := max(1, max(color[0], max(color[1], color[2])))
		for c := range color {
			lightmap.Corners[index][c] = max(0, float32(math.Round(float64(color[c]/scale)*64)/64))
		}
	}
	return lightmap, len(sources)
}

// Mirrors tg light_source/falloff_at_coord, including its 30-degree cone edge.
func (light lightSource) falloff(x, y float64) float64 {
	value := 1 - min(1, math.Sqrt(max(0, x*x+y*y+light.height))/max(1, light.radius))
	if light.angle <= 0 || light.angle >= 360 || (x == 0 && y == 0) {
		return value
	}
	dx, dy := 0.0, 0.0
	if light.dir&1 != 0 {
		dy++
	}
	if light.dir&2 != 0 {
		dy--
	}
	if light.dir&4 != 0 {
		dx++
	}
	if light.dir&8 != 0 {
		dx--
	}
	if dx == 0 && dy == 0 {
		return value
	}
	cos := (x*dx + y*dy) / (math.Hypot(x, y) * math.Hypot(dx, dy))
	angle := math.Acos(max(-1, min(1, cos))) * 180 / math.Pi
	return max(0, value*(1-max(0, angle-light.angle/2)/30))
}

// Conservative grid visibility prevents light leaking through opaque corners.
// BYOND's view() and runtime directional-opacity effects can differ from this estimate.
func lineVisible(opaque []bool, w, h, x0, y0, x1, y1 int) bool {
	blocked := func(x, y int) bool { return x < 0 || y < 0 || x >= w || y >= h || opaque[y*w+x] }
	dx, dy := x1-x0, y1-y0
	nx, ny := int(math.Abs(float64(dx))), int(math.Abs(float64(dy)))
	sx, sy := 1, 1
	if dx < 0 {
		sx = -1
	}
	if dy < 0 {
		sy = -1
	}
	x, y, ix, iy := x0, y0, 0, 0
	for ix < nx || iy < ny {
		a, b := (1+2*ix)*ny, (1+2*iy)*nx
		if a == b {
			if blocked(x+sx, y) && blocked(x, y+sy) {
				return false
			}
			x += sx
			y += sy
			ix++
			iy++
		} else if a < b {
			x += sx
			ix++
		} else {
			y += sy
			iy++
		}
		if blocked(x, y) {
			return false
		}
	}
	return true
}
