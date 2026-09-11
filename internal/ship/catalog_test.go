package ship

import (
	"os"
	"path/filepath"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmvars"
)

func TestDiscoverListsFixedShipsBesideModularOnes(t *testing.T) {
	root := t.TempDir()
	env := &dmenv.Dme{RootDir: root, RootFile: filepath.Join(root, "test.dme"), Objects: map[string]*dmenv.Object{}}
	add := func(path string, values map[string]string) {
		v := dmvars.MutableVariables{}
		for k, val := range values {
			v.Put(k, val)
		}
		env.Objects[path] = &dmenv.Object{Path: path, Vars: v.ToImmutable()}
	}
	if err := os.MkdirAll(filepath.Join(root, "config"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config/modules.toml"), []byte("directory = \"_maps/voidcrew/ship_modules/\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	add(HullType, map[string]string{"name": `"ships"`, "prefix": `"_maps/voidcrew/ships/"`, "port_id": `"ship"`})
	add(SlotMarker, map[string]string{"config_file": `"config/modules.toml"`})
	add(HullType+"/box", map[string]string{"name": `"Box-class"`, "suffix": `"box"`})
	add(HullType+"/box/variant", map[string]string{"name": `"Box variant"`, "suffix": `"box_variant"`})
	add(HullType+"/hidden", map[string]string{"name": `"Hidden"`, "suffix": `"hidden"`, "player_hidden": "1"})
	add(HullType+"/abstract_base", map[string]string{"name": `"Abstract"`, "suffix": `"abstract"`, "abstract": HullType + "/abstract_base"})
	add(HullType+"/unmapped", map[string]string{"name": `"Unmapped"`})
	add(HullType+"/scarab", map[string]string{"name": `"Scarab-class"`, "suffix": `"scarab_a"`, "has_upgrade_slots": "1", "upgrade_slot_ids": `list("scarab_med")`})
	add("/datum/ship_upgrade_module/scarab_med_basic", map[string]string{"id": `"med_basic"`, "name": `"Medbay"`, "slot": `"scarab_med"`, "for_ship": HullType + "/scarab", "map_file": `"scarab/med_basic.dmm"`, "is_default": "1"})
	add(HullType+"/lonely", map[string]string{"name": `"Lonely"`, "suffix": `"lonely"`, "has_upgrade_slots": "1", "upgrade_slot_ids": `list("bay")`})
	add(HullType+"/misregistered", map[string]string{"name": `"Misregistered"`, "suffix": `"misregistered"`})
	add("/datum/ship_upgrade_module/misregistered_bay", map[string]string{"id": `"bay"`, "name": `"Bay"`, "slot": `"bay"`, "for_ship": HullType + "/misregistered", "map_file": `"misregistered/bay.dmm"`})
	catalog, err := Discover(env)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Hull{}
	for _, h := range catalog.Hulls {
		got[h.Type] = h
	}
	if len(got) != 3 {
		t.Fatalf("unexpected fleet: %+v", catalog.Hulls)
	}
	if box := got[HullType+"/box"]; !box.Fixed || box.Suffix != "box" || len(box.Slots) != 0 || len(box.Modules) != 0 {
		t.Fatalf("fixed ship not listed as fixed: %+v", box)
	}
	if scarab := got[HullType+"/scarab"]; scarab.Fixed || len(scarab.Modules) != 1 || len(scarab.Slots) != 1 {
		t.Fatalf("modular ship changed: %+v", scarab)
	}
	if lonely := got[HullType+"/lonely"]; lonely.Fixed || len(lonely.Slots) != 1 {
		t.Fatalf("modular ship without rooms was not listed: %+v", lonely)
	}
	if catalog.Hulls[0].Name != "Box-class" || catalog.Hulls[1].Name != "Lonely" {
		t.Fatalf("fleet is not sorted by name: %+v", catalog.Hulls)
	}
}
