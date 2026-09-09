package planet

import (
	"math"
	"testing"
)

func TestClimateBandsAndPreviewProvenance(t *testing.T) {
	for _, caves := range []bool{false, true} {
		ends := []float64{.2, .4, .6, .65, .8}
		if caves {
			ends = []float64{.25, .5, .75}
		}
		for row, end := range ends {
			if at := ClimateAt(end, .4, caves); at.Row != row || at.Col != 1 || at.Caves != caves {
				t.Fatalf("wrong inclusive boundary: %+v", at)
			}
			if at := ClimateAt(math.Nextafter(end, 1), math.Nextafter(.4, 1), caves); at.Row != row+1 || at.Col != 2 {
				t.Fatalf("wrong upper neighbor: %+v", at)
			}
		}
	}
	c := fixture(t)
	s := NewState(c, definition(t, c, "/datum/planet/test"))
	s.Size = 48
	for _, caves := range []bool{false, true} {
		p, err := Generate(c, s, c.Dme, PreviewOptions{Caves: caves})
		if err != nil {
			t.Fatal(err)
		}
		for _, cell := range p.Cells {
			isCave := caves || cell.Height > s.Definition.Settings.Mountain
			want := ClimateAt(cell.Heat, cell.Moisture, isCave)
			if cell.Climate != want {
				t.Fatalf("tile lost its source rule: %+v", cell)
			}
			grid := s.Definition.Surface
			if want.Caves {
				grid = s.Definition.Caves
			}
			if cell.Biome != grid[want.Row][want.Col] {
				t.Fatal("preview provenance differs from generator lookup")
			}
		}
	}
}
