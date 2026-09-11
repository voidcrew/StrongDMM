package ship

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/dmapi/dmvars"
	"sdmm/internal/util"
	"sdmm/third_party/sdmmparser"
)

func TestRenameComponentsSaveAndUndo(t *testing.T) {
	for _, handwritten := range []bool{false, true} {
		t.Run(map[bool]string{false: "authored", true: "handwritten"}[handwritten], func(t *testing.T) {
			p, sourceFile := loadedRoomProject(t, true)
			if !handwritten {
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
			beforeSource, err := os.ReadFile(sourceFile)
			if err != nil {
				t.Fatal(err)
			}
			before := p.Capture()
			for _, attempt := range []struct{ scope, name string }{
				{"theme/standard", " OTHER   variant "},
				{"theme/standard", " \t "},
				{"module/" + module.ID, " empty ROOM "},
				{"module/missing", "Medical"},
			} {
				if err := p.Rename(attempt.scope, attempt.name); err == nil {
					t.Fatalf("accepted invalid rename: %+v", attempt)
				}
			}
			if err := p.Rename("theme/standard", p.Hull.Themes[0].Name); err != nil || p.Modified() {
				t.Fatalf("keeping the current name changed the project: %v", err)
			}
			if err := p.Rename("theme/standard", "  Scout [A]  "); err != nil {
				t.Fatal(err)
			}
			if err := p.Rename("module/"+module.ID, `Medical "Bay"`); err != nil {
				t.Fatal(err)
			}
			if !p.Modified() {
				t.Fatal("rename was not marked dirty")
			}
			expected := cloneHull(before.Hull)
			expected.Themes[0].Name = "Scout [A]"
			expected.Suffix = "loaded_rooms_scout_a"
			expected.Themes[0].Suffix = expected.Suffix
			expected.Modules[0].Name = `Medical "Bay"`
			expected.Modules[0].File = "loaded_rooms/medical_bay.dmm"
			if !reflect.DeepEqual(expected, p.Hull) {
				t.Fatal("rename changed component IDs, slots or unrelated metadata")
			}
			changes, err := p.Changes()
			if err != nil {
				t.Fatal(err)
			}
			moved := false
			for _, change := range changes {
				if change.Delete && strings.HasSuffix(change.Path, ".dmm") {
					moved = true
				}
			}
			if !moved {
				t.Fatal("rename did not remove the old map paths")
			}
			if err = p.Save(); err != nil || p.Modified() {
				t.Fatalf("rename did not save cleanly: %v", err)
			}
			saved, _ := os.ReadFile(sourceFile)
			wantSource := bytes.Replace(beforeSource, []byte("name = "+dmQuote(before.Hull.Themes[0].Name)), []byte("name = "+dmQuote(expected.Themes[0].Name)), 1)
			wantSource = bytes.Replace(wantSource, []byte("name = "+dmQuote(module.Name)), []byte("name = "+dmQuote(expected.Modules[0].Name)), 1)
			wantSource = bytes.Replace(wantSource, []byte("template_suffix = "+dmQuote(before.Hull.Themes[0].Suffix)), []byte("template_suffix = "+dmQuote(expected.Themes[0].Suffix)), 1)
			wantSource = bytes.Replace(wantSource, []byte("map_file = "+dmQuote(module.File)), []byte("map_file = "+dmQuote(expected.Modules[0].File)), 1)
			if !bytes.Equal(saved, wantSource) {
				t.Fatal("save changed more than the names and map references")
			}
			reopened, err := OpenProject(p.Catalog, p.Dme, p.Hull)
			if err != nil || !reflect.DeepEqual(reopened.Hull, expected) || reopened.Modified() {
				t.Fatalf("saved names did not reopen: %v", err)
			}
			after := p.Capture()
			p.Restore(before)
			if err = p.Save(); err != nil {
				t.Fatal(err)
			}
			restored, _ := os.ReadFile(sourceFile)
			if !bytes.Equal(restored, beforeSource) {
				t.Fatal("undo after Save did not restore the original source")
			}
			p.Restore(after)
			if err = p.Save(); err != nil {
				t.Fatal(err)
			}
			redone, _ := os.ReadFile(sourceFile)
			if !bytes.Equal(redone, saved) {
				t.Fatal("redo after Save did not restore the renamed source")
			}
			// New room options on loaded ships use generated registrations, too.
			if err = p.addRect(0, "new_room", "New Room", util.Point{X: 10, Y: 7, Z: 1}, util.Point{X: 12, Y: 9, Z: 1}); err != nil {
				t.Fatal(err)
			}
			if err = p.Rename("module/new_room_basic", "New Infirmary"); err != nil {
				t.Fatal(err)
			}
			if err = p.Rename("module/new_room_basic", " medical   \"BAY\" "); err == nil {
				t.Fatal("accepted a duplicate room name")
			}
			if err = p.Save(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRewriteNamePreservesSourceAndRejectsAmbiguity(t *testing.T) {
	const path = "/datum/ship_theme/test"
	for _, newline := range []string{"\n", "\r\n"} {
		before := strings.ReplaceAll("// name = \"Example\"\n"+path+"\n\tname = \"Original\" // Keep this\n\tpart_cost = list(\"misc\" = 3)\n\n"+path+"/custom_proc()\n\treturn 17\n", "\n", newline)
		after, err := rewriteName([]byte(before), path, "Original", `New [Bay]`)
		if err != nil || string(after) != strings.Replace(before, `"Original"`, dmQuote(`New [Bay]`), 1) {
			t.Fatalf("literal rename did not preserve the source: %v", err)
		}
		inherited := path + newline + "\tid = \"test\"" + newline
		after, err = rewriteName([]byte(inherited), path, "Inherited", "Local")
		if err != nil || string(after) != path+newline+"\tname = \"Local\""+newline+"\tid = \"test\""+newline {
			t.Fatalf("inherited name could not be overridden: %v", err)
		}
	}
	for _, block := range []string{
		"\tname = NAME_MACRO\n",
		"\tname = \"Original\" + \" suffix\"\n",
		"\tname = \"Original\"\n\tname = \"Again\"\n",
		"#ifdef TEST\n\tname = \"Original\"\n#endif\n",
		"\tname = \"Different\"\n",
	} {
		if _, err := rewriteName([]byte(path+"\n"+block), path, "Original", "Changed"); err == nil {
			t.Fatalf("accepted ambiguous name source: %q", block)
		}
	}
}
