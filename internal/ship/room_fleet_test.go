package ship

import (
	"os"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmicon"
	"sdmm/internal/dmapi/dmmap"
)

// Read-only compatibility check against the real fleet's handwritten layouts.
func TestFleetRoomRegistrationSources(t *testing.T) {
	file := os.Getenv("SHIP_RENDER_TEST_DME")
	if file == "" {
		t.Skip("set SHIP_RENDER_TEST_DME for fleet source checks")
	}
	env, err := dmenv.New(file)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := Discover(env)
	if err != nil {
		t.Fatal(err)
	}
	checked, fixed := 0, 0
	for _, hull := range catalog.Hulls {
		if hull.Fixed {
			fixed++
		}
		project, err := OpenProject(catalog, env, hull)
		if err != nil {
			t.Fatal(err)
		}
		if project.Settings != nil {
			continue
		}
		themes := hull.Themes
		if len(themes) == 0 {
			themes = []Theme{{}}
		}
		for _, theme := range themes {
			if err = project.prepareRooms(&theme); err != nil {
				t.Fatalf("%s / %s: %v", hull.Name, theme.Name, err)
			}
			typePath := project.rooms.targets[theme.ID]
			source, err := project.roomTypeFile(typePath)
			if err != nil {
				t.Fatal(err)
			}
			before := roomSlots(project.rooms.base, theme.ID)
			after := append(append([]string{}, hull.SlotsFor(theme)...), "workshop_source_check")
			if _, err = rewriteRoomSlots(project.rooms.sources[source].Before, typePath, before, after); err != nil {
				t.Fatalf("%s / %s: %v", hull.Name, theme.Name, err)
			}
			checked++
		}
		if project.Modified() {
			t.Fatal("reading room definitions marked a ship dirty")
		}
	}
	if checked == 0 {
		t.Fatal("no loaded ship room definitions checked")
	}
	t.Logf("checked %d fleet room-list definitions (%d fixed ships) without writing them", checked, fixed)
}

// Every fleet ship opens read-only, including old maps that reference types
// the environment no longer defines.
func TestFleetShipsAssemble(t *testing.T) {
	file := os.Getenv("SHIP_RENDER_TEST_DME")
	if file == "" {
		t.Skip("set SHIP_RENDER_TEST_DME for fleet assembly checks")
	}
	env, err := dmenv.New(file)
	if err != nil {
		t.Fatal(err)
	}
	dmmap.PrefabStorage.Free()
	dmicon.Cache.Free()
	dmmap.Init(env)
	catalog, err := Discover(env)
	if err != nil {
		t.Fatal(err)
	}
	withUnknown := 0
	for _, hull := range catalog.Hulls {
		project, err := OpenProject(catalog, env, hull)
		if err != nil {
			t.Fatal(err)
		}
		themes := hull.Themes
		if len(themes) == 0 {
			themes = []Theme{{}}
		}
		for _, theme := range themes {
			selected := map[string]string{}
			for _, slot := range hull.SlotsFor(theme) {
				for _, m := range hull.Modules {
					if m.Slot == slot && m.Default && m.Available(theme.ID) {
						selected[slot] = m.ID
					}
				}
			}
			a, err := project.Assemble(theme, selected)
			if err != nil {
				t.Fatalf("%s / %s: %v", hull.Name, theme.Name, err)
			}
			if _, err = a.Display(env); err != nil {
				t.Fatalf("%s / %s preview: %v", hull.Name, theme.Name, err)
			}
		}
		if project.Modified() {
			t.Fatalf("%s: opening marked the ship dirty", hull.Name)
		}
		for path, d := range project.Documents {
			if len(d.Unknown) > 0 {
				withUnknown++
				t.Logf("%s keeps %d unknown types: %v", path, len(d.Unknown), d.Unknown)
			}
		}
	}
	t.Logf("assembled %d fleet ships; %d maps keep unknown types", len(catalog.Hulls), withUnknown)
}

// Read-only compatibility check: every handwritten fleet variant can be found
// and rewritten where it is declared, including inherited availability lists.
func TestFleetVariantRegistrationSources(t *testing.T) {
	file := os.Getenv("SHIP_RENDER_TEST_DME")
	if file == "" {
		t.Skip("set SHIP_RENDER_TEST_DME for fleet source checks")
	}
	env, err := dmenv.New(file)
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := Discover(env)
	if err != nil {
		t.Fatal(err)
	}
	variants, inherited := 0, 0
	for _, hull := range catalog.Hulls {
		project, err := OpenProject(catalog, env, hull)
		if err != nil {
			t.Fatal(err)
		}
		if project.Settings != nil || len(hull.Themes) == 0 {
			continue
		}
		if err = project.prepareRooms(nil); err != nil {
			t.Fatalf("%s: %v", hull.Name, err)
		}
		if _, err = project.themeRegistrationFile(); err != nil {
			t.Fatalf("%s: %v", hull.Name, err)
		}
		if err = project.prepareAvailableThemes(); err != nil {
			t.Fatalf("%s: %v", hull.Name, err)
		}
		if err = project.prepareModuleThemes(); err != nil {
			t.Fatalf("%s: %v", hull.Name, err)
		}
		for _, m := range hull.Modules {
			target := project.rooms.moduleThemes[m.ID]
			if target.shared != target.own {
				inherited++
			}
			if _, err = rewriteLiteralList(project.rooms.sources[target.shared.file].Before, target.shared.typePath, "for_theme", dmList(m.Themes)); err != nil {
				t.Fatalf("%s / %s: %v", hull.Name, m.Name, err)
			}
		}
		for _, theme := range hull.Themes {
			if err = project.prepareThemeDefinition(theme.ID); err != nil {
				t.Fatalf("%s / %s: %v", hull.Name, theme.Name, err)
			}
			target := project.rooms.themeTargets[theme.ID]
			if _, err = rewriteFlag(project.rooms.sources[target.file].Before, target.typePath, "is_default", theme.Default); err != nil {
				t.Fatalf("%s / %s: %v", hull.Name, theme.Name, err)
			}
			_, removed, err := removeDefinitions(project.rooms.sources[target.file].Before, []string{target.typePath})
			if err != nil || !removed[target.typePath] {
				t.Fatalf("%s / %s cannot be removed: %v", hull.Name, theme.Name, err)
			}
			variants++
		}
		if project.Modified() {
			t.Fatalf("%s: reading variant definitions marked the ship dirty", hull.Name)
		}
	}
	if variants == 0 {
		t.Fatal("no fleet variants checked")
	}
	t.Logf("checked %d fleet variants; %d room options inherit a shared availability list", variants, inherited)
}
