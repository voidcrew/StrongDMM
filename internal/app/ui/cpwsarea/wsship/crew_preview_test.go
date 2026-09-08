package wsship

import "testing"

func TestCrewPreviewTintPreservesGreyscaleAndAlpha(t *testing.T) {
	v := crewVisual{layers: []crewLayer{{color: 0xffffffff}, {color: crewColor("#80ff40")}}}
	v.tintSince(1, crewColor("#ff804080"))
	if v.layers[0].color != 0xffffffff || v.layers[1].color != 0x80108080 {
		t.Fatalf("incorrect color/alpha multiplication: %#x", v.layers[1].color)
	}
}
