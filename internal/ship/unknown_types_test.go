package ship

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmmap"
	"sdmm/internal/dmapi/dmmap/dmmdata"
	"sdmm/internal/dmapi/dmmap/dmmdata/dmmprefab"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
)

// Ships keep opening when a map references a type the environment no longer
// defines, and those atoms are saved back untouched rather than discarded.
func TestShipMapsWithUnknownTypesOpenAndKeepThem(t *testing.T) {
	c, env := authorEnvironment(t)
	p, err := NewProject(c, env, "relic_ship", "Relic Ship", 16, 16)
	if err != nil {
		t.Fatal(err)
	}
	theme := p.Hull.Themes[0]
	if err = p.Deck(theme, util.Point{X: 2, Y: 2, Z: 1}, util.Point{X: 12, Y: 12, Z: 1}); err != nil {
		t.Fatal(err)
	}
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	hullFile, _ := c.HullFile(p.Hull, theme)
	data, err := dmmdata.New(hullFile)
	if err != nil {
		t.Fatal(err)
	}
	relicAt := util.Point{X: 4, Y: 5, Z: 1}
	vars := dmvars.MutableVariables{}
	vars.Put("name", `"ancient relic"`)
	relic := dmmprefab.New(0, "/obj/item/relic/removed", vars.ToImmutable())
	data.Dictionary["zzz"] = append(dmmdata.Prefabs{relic}, data.Dictionary[data.Grid[relicAt]]...)
	data.Grid[relicAt] = "zzz"
	if err = os.WriteFile(hullFile, data.EncodeTGM(), 0600); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(hullFile)

	c.Hulls = []Hull{p.Hull}
	p, err = OpenProject(c, env, p.Hull)
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Assemble(theme, nil)
	if err != nil {
		t.Fatalf("ship with an unknown type did not open: %v", err)
	}
	if p.Modified() {
		t.Fatal("opening marked the ship dirty")
	}
	doc := p.Documents[hullFile]
	if len(doc.Unknown) != 1 || doc.Unknown[0] != "/obj/item/relic/removed" {
		t.Fatalf("unknown types not reported: %v", doc.Unknown)
	}
	found := false
	for _, i := range doc.Map.GetTile(relicAt).Instances() {
		if i.Prefab().Path() == "/obj/item/relic/removed" {
			found = true
		}
	}
	if !found {
		t.Fatal("unknown atom was dropped from the tile")
	}
	warned := false
	for _, issue := range a.Issues {
		if strings.Contains(issue.Message, "/obj/item/relic/removed") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("review checks do not mention the unknown type: %+v", a.Issues)
	}
	if _, err = a.Display(env); err != nil {
		t.Fatalf("preview failed with an unknown type: %v", err)
	}
	// An unrelated edit saves the map with the unknown atom still in place.
	doc.Map.GetTile(util.Point{X: 8, Y: 8, Z: 1}).InstancesAdd(dmmap.PrefabStorage.Initial("/obj/item/test"))
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(hullFile)
	if !bytes.Contains(saved, []byte("/obj/item/relic/removed{")) || !bytes.Contains(saved, []byte(`name = "ancient relic"`)) {
		t.Fatalf("save discarded the unknown atom:\n%s", saved)
	}
	if bytes.Equal(saved, original) {
		t.Fatal("the edit was not saved")
	}
	// Extracting a room that contains the unknown atom keeps it in the room map.
	if err = p.addRect(0, "relic_bay", "Relic Bay", util.Point{X: 3, Y: 4, Z: 1}, util.Point{X: 5, Y: 6, Z: 1}); err != nil {
		t.Fatal(err)
	}
	if err = p.Save(); err != nil {
		t.Fatal(err)
	}
	module, err := p.ModuleSource(p.Hull.Modules[0], theme.ID)
	if err != nil {
		t.Fatal(err)
	}
	room, _ := os.ReadFile(module)
	hull, _ := os.ReadFile(hullFile)
	if !bytes.Contains(room, []byte("/obj/item/relic/removed{")) || bytes.Contains(hull, []byte("/obj/item/relic/removed")) {
		t.Fatal("unknown atom did not move into the extracted room")
	}
}
