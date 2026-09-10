package planet

import (
	"math"
	"testing"
)

func TestClimateBiasReshapesAndReadsBack(t *testing.T) {
	base := DefaultHeatShares[:]
	if got := BiasedShares(base, 0); !equalShares(got, base) {
		t.Fatal("zero bias changed the defaults", got)
	}
	hot := BiasedShares(base, 1)
	cold := BiasedShares(base, -1)
	if hot[5] <= hot[0] || cold[0] <= cold[5] || hot[5] < 40 || cold[0] < 40 {
		t.Fatal("bias did not favour the chosen end", hot, cold)
	}
	total := 0.0
	for _, share := range hot {
		total += share
	}
	if math.Abs(total-100) > .5 {
		t.Fatal("biased shares do not total 100", total)
	}
	for _, bias := range []float64{-.8, -.3, 0, .25, .6, 1} {
		if got := ShareBias(BiasedShares(base, bias), base); math.Abs(got-bias) > .02 {
			t.Fatalf("bias %v read back as %v", bias, got)
		}
	}
	if got := ShareBias([]float64{100, 0, 0, 0, 0, 0}, base); got != -1 {
		t.Fatal("an all-cold planet should sit at the cold edge", got)
	}
	if got := ShareBias([]float64{0, 0, 0, 0, 0, 0}, base); math.Abs(got) > .1 {
		t.Fatal("empty shares should read near the centre", got)
	}
	wet := BiasedShares(DefaultMoistureShares[:], .5)
	if wet[4] <= wet[0] || len(wet) != 5 {
		t.Fatal("moisture bias did not favour wet bands", wet)
	}
}

func equalShares(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > .05 {
			return false
		}
	}
	return true
}
