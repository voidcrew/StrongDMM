package ship

import (
	"os"
	"testing"

	"sdmm/internal/dmapi/dmenv"
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
	checked := 0
	for _, hull := range catalog.Hulls {
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
	t.Logf("checked %d fleet room-list definitions without writing them", checked)
}
