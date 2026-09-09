package mappreview

import (
	"sdmm/internal/dmapi/dm"
	"sdmm/internal/dmapi/dmmap"
)

// Exterior light is a preview aid. Only space connected to the map boundary
// contributes light; sealed rooms and interior space pockets keep their lighting.
func illuminateExterior(source *dmmap.Dmm, lighting *Lighting, opaque []bool) {
	w, h := lighting.Width, lighting.Height
	if w == 0 || h == 0 {
		return
	}
	void := make([]bool, w*h)
	for i := range void {
		void[i] = true // Empty assembly cells are outside the ship, too.
	}
	for _, tile := range source.Tiles {
		index := (tile.Coord.Y-1)*w + tile.Coord.X - 1
		for _, instance := range tile.Instances() {
			path := instance.Prefab().Path()
			if dm.IsPath(path, "/turf") {
				void[index] = dm.IsPath(path, "/turf/open/space") || dm.IsPath(path, "/turf/template_noop")
			}
		}
	}
	illuminate := func(x, y int) {
		lighting.Corners[y*(w+1)+x] = RGB{.85, .85, .85}
	}
	visited := make([]bool, w*h)
	queue := make([]int, 0)
	enqueue := func(x, y int) {
		if x < 0 || x >= w || y < 0 || y >= h {
			return
		}
		index := y*w + x
		if !void[index] || opaque[index] || visited[index] {
			return
		}
		visited[index] = true
		queue = append(queue, index)
	}
	// Treat the space beyond a tightly cropped map as lit, including when
	// the hull touches the map edge. Shared corners light its outside face.
	for x := 0; x <= w; x++ {
		illuminate(x, 0)
		illuminate(x, h)
		if x < w {
			enqueue(x, 0)
			enqueue(x, h-1)
		}
	}
	for y := 0; y <= h; y++ {
		illuminate(0, y)
		illuminate(w, y)
		if y < h {
			enqueue(0, y)
			enqueue(w-1, y)
		}
	}
	for head := 0; head < len(queue); head++ {
		index := queue[head]
		x, y := index%w, index/w
		illuminate(x, y)
		illuminate(x+1, y)
		illuminate(x, y+1)
		illuminate(x+1, y+1)
		enqueue(x-1, y)
		enqueue(x+1, y)
		enqueue(x, y-1)
		enqueue(x, y+1)
	}
}
