package planet

import (
	"bytes"
	"math"
	"os"
	"testing"
)

func TestClimateSharesShapeBands(t *testing.T) {
	g := Generator{}
	if g.HeatShares() != DefaultHeatShares || g.CaveHeatShares() != DefaultCaveHeatShares || g.MoistureShares() != DefaultMoistureShares {
		t.Fatal("unset shares must read as the built-in cut points")
	}
	g.Heat = [6]float64{100, 0, 0, 0, 0, 0}
	if at := ClimateAt(g, .99, .5, false); at.Row != 0 || at.Col != 2 {
		t.Fatal("a single hot share should still leave every tile coldest", at)
	}
	g.Moisture = [5]float64{0, 0, 0, 0, 100}
	if at := ClimateAt(g, .5, .01, false); at.Col != 4 {
		t.Fatal("empty dry bands should never appear", at)
	}
	g.CaveHeat = [4]float64{10, 10, 10, 70}
	if ClimateAt(g, .3, .5, true).Row != 2 || ClimateAt(g, .31, .5, true).Row != 3 {
		t.Fatal("cave shares did not move the boundary")
	}
	shares := []float64{20, 20, 20, 5, 15, 20}
	BalanceShares(shares, 3, 50)
	total := 0.0
	for _, share := range shares {
		total += share
	}
	if shares[3] != 50 || math.Abs(total-100) > .5 || math.Abs(shares[0]-10.5) > .1 || math.Abs(shares[4]-7.9) > .1 {
		t.Fatal("balancing did not scale the other bands", shares)
	}
	BalanceShares(shares, 0, 100)
	if shares[0] != 100 || shares[1] != 0 || shares[5] != 0 {
		t.Fatal("a full share should empty the others", shares)
	}
	BalanceShares(shares, 2, 30)
	if shares[2] != 30 || shares[0] != 70 || shares[1] != 0 {
		t.Fatal("the only remaining band should absorb the remainder", shares)
	}
	shares = []float64{0, 0, 0, 0, 0, 0}
	BalanceShares(shares, 2, 30)
	if shares[2] != 30 || math.Abs(shares[0]-14) > .1 || math.Abs(shares[5]-14) > .1 {
		t.Fatal("empty bands should split the remainder evenly", shares)
	}
}

func TestClimateSharesSaveAndReopen(t *testing.T) {
	c := fixture(t)
	p, e := Open(c, definition(t, c, "/datum/planet/test"))
	if e != nil {
		t.Fatal(e)
	}
	p.State.Definition.Settings.Heat = [6]float64{40, 30, 10, 5, 5, 10}
	if e = p.Save(); e != nil {
		t.Fatal(e)
	}
	code, _ := os.ReadFile(p.GeneratedPath())
	if !bytes.Contains(code, []byte("\theat_shares = list(40, 30, 10, 5, 5, 10)\n")) || bytes.Contains(code, []byte("humidity_shares")) || bytes.Contains(code, []byte("cave_heat_shares")) {
		t.Fatalf("generator shares were not written, or defaults were:\n%s", code)
	}
	c2 := load(t, c.Dme.RootFile)
	p2, e := Open(c2, definition(t, c2, "/datum/planet/test"))
	if e != nil {
		t.Fatal(e)
	}
	if p2.State.Definition.Settings.Heat != p.State.Definition.Settings.Heat || p2.State.Definition.Settings.Moisture != DefaultMoistureShares || p2.Modified() {
		t.Fatal("shares did not reopen from the generated generator", p2.State.Definition.Settings)
	}
	if at := ClimateAt(p2.State.Definition.Settings, .5, .5, false); at.Row != 1 {
		t.Fatal("reopened shares are not used for lookup", at)
	}
	p2.State.Definition.Settings.Moisture = [5]float64{-1, 0, 0, 0, 0}
	if e = p2.State.Validate(c2); e == nil {
		t.Fatal("negative share accepted")
	}
	p2.State.Definition.Settings.Moisture = [5]float64{}
	if e = p2.State.Validate(c2); e != nil {
		t.Fatal("unset shares should validate as defaults", e)
	}
}
