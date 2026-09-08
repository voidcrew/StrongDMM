package ship

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/third_party/sdmmparser"
)

func TestDescriptionsSaveAndUndo(t *testing.T) {
	for _, authored := range []bool{false, true} {
		t.Run(map[bool]string{true: "authored", false: "handwritten"}[authored], func(t *testing.T) {
			p, sourceFile := loadedRoomProject(t, true)
			if authored {
				p.Settings = &Settings{Version: 1, ID: "loaded_rooms", Crew: 1, PortDirection: 1}
				for _, path := range p.outputPaths() {
					if err := p.track(path); err != nil {
						t.Fatal(err)
					}
				}
				if err := p.Save(); err != nil {
					t.Fatal(err)
				}
			}
			module := p.Hull.Modules[0]
			vars := dmvars.MutableVariables{}
			vars.Put("id", dmQuote(module.ID))
			vars.Put("for_ship", p.Hull.Type)
			typePath := "/datum/ship_upgrade_module/loaded_rooms_" + module.ID
			p.Dme.Objects[typePath] = &dmenv.Object{Path: typePath, Vars: vars.ToImmutable(), Location: sdmmparser.Location{File: sourceFile}}
			before := p.Capture()
			original, err := os.ReadFile(sourceFile)
			if err != nil {
				t.Fatal(err)
			}
			want := "A \"medical\" ship [A].\nEquipment: medicine \\ supplies."
			for _, scope := range []string{"theme/standard", "module/" + module.ID} {
				if err = p.SetDescription(scope, want); err != nil {
					t.Fatal(err)
				}
			}
			if err = p.Rename("theme/standard", "Medical ship"); err != nil {
				t.Fatal(err)
			}
			if !p.Modified() {
				t.Fatal("description edit has no dirty state")
			}
			if err = p.Save(); err != nil || p.Modified() {
				t.Fatalf("save: %v", err)
			}
			saved, _ := os.ReadFile(sourceFile)
			if bytes.Count(saved, []byte("desc = "+dmQuote(want))) != 2 {
				t.Fatal("descriptions missing from DM registrations")
			}
			if !authored && !bytes.Contains(saved, []byte("custom_proc")) {
				t.Fatal("handwritten code was lost")
			}
			expected := cloneHull(p.Hull)
			reopened, err := OpenProject(p.Catalog, p.Dme, p.Hull)
			if err != nil || !reflect.DeepEqual(reopened.Hull, expected) || reopened.Modified() {
				t.Fatalf("reopen: %v", err)
			}
			after := p.Capture()
			p.Restore(before)
			if err = p.Save(); err != nil {
				t.Fatal(err)
			}
			restored, _ := os.ReadFile(sourceFile)
			if !bytes.Equal(original, restored) {
				t.Fatal("undo did not restore original source")
			}
			p.Restore(after)
			if err = p.Save(); err != nil {
				t.Fatal(err)
			}
			redone, _ := os.ReadFile(sourceFile)
			if !bytes.Equal(saved, redone) {
				t.Fatal("redo did not restore descriptions")
			}
			if err = p.SetDescription("module/"+module.ID, ""); err != nil {
				t.Fatal(err)
			}
			if err = p.Save(); err != nil {
				t.Fatal(err)
			}
			cleared, _ := os.ReadFile(sourceFile)
			if !bytes.Contains(cleared, []byte(`desc = ""`)) {
				t.Fatal("blank description did not override previous value")
			}
		})
	}
}

func TestDescriptionSourceContinuations(t *testing.T) {
	const path = "/datum/ship_theme/test"
	for _, newline := range []string{"\n", "\r\n"} {
		literal := "\"A long \\\n\t\tdescription.\""
		before := strings.ReplaceAll(path+"\n\tname = \"Name\"\n\tdesc = "+literal+" // Keep this comment\n\tpart_cost = list(\"misc\" = 3)\n", "\n", newline)
		literal = strings.ReplaceAll(literal, "\n", newline)
		after, err := rewriteTextField([]byte(before), path, "desc", "A long description.", "New [description]\nSecond line")
		if err != nil || string(after) != strings.Replace(before, literal, dmQuote("New [description]\nSecond line"), 1) {
			t.Fatalf("continued description was not replaced safely: %v", err)
		}
	}
}

// Check real inherited and continued descriptions without changing fleet files.
func TestFleetDescriptionSources(t *testing.T) {
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
		p, err := OpenProject(catalog, env, hull)
		if err != nil {
			t.Fatal(err)
		}
		if p.Settings != nil {
			continue
		}
		var scopes []string
		for _, theme := range hull.Themes {
			scopes = append(scopes, "theme/"+theme.ID)
		}
		for _, module := range hull.Modules {
			scopes = append(scopes, "module/"+module.ID)
		}
		for _, scope := range scopes {
			if err = p.SetDescription(scope, "Updated shipyard description."); err != nil {
				t.Fatalf("%s / %s: %v", hull.Name, scope, err)
			}
			checked++
		}
		if _, err = p.Changes(); err != nil {
			t.Fatalf("%s: %v", hull.Name, err)
		}
	}
	if checked == 0 {
		t.Fatal("no fleet descriptions checked")
	}
	t.Logf("checked %d variant and module descriptions without writing them", checked)
}
