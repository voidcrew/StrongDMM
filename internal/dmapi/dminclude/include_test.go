package dminclude

import (
	"strings"
	"testing"
)

// A trimmed tgstation.dme: a code group, the voidcrew\mapping group with direct
// files before child folders, then unrelated groups and the END_INCLUDE marker.
const sample = `// DM Environment file for tgstation.dme.
// All manual changes should be made outside the BEGIN_ and END_ blocks.

// BEGIN_FILE_DIR
#define FILE_DIR .
// END_FILE_DIR

// BEGIN_PREFERENCES
#define DEBUG
// END_PREFERENCES

// BEGIN_INCLUDE
#include "code\_compile_options.dm"
#include "code\world.dm"
#include "voidcrew\datums\mapgen\planet_rivers.dm"
#include "voidcrew\datums\mapgen\planet_settings.dm"
#include "voidcrew\mapping\_mapping.dm"
#include "voidcrew\mapping\areas.dm"
#include "voidcrew\mapping\rivers.dm"
#include "voidcrew\mapping\docking_port\_docking_port.dm"
#include "voidcrew\mapping\shuttles\_shuttle.dm"
#include "voidcrew\mapping\shuttles\box.dm"
#include "voidcrew\mapping\shuttles\delta.dm"
#include "voidcrew\mapping\shuttles\scarab.dm"
#include "voidcrew\mapping\shuttles\nanotrasen\bead.dm"
#include "voidcrew\mapping\spawners\nanites.dm"
#include "voidcrew\modules\ship_upgrades\_ship_upgrades.dm"
#include "voidcrew\modules\ship_upgrades\admin_verbs.dm"
#include "voidcrew\turfs\open\planet.dm"
// END_INCLUDE
`

func lines(s string) []string { return strings.Split(strings.TrimRight(s, "\r\n"), "\n") }

func index(t *testing.T, text, want string) int {
	t.Helper()
	at := -1
	for i, line := range lines(text) {
		if strings.TrimRight(line, "\r") == want {
			if at >= 0 {
				t.Fatalf("duplicate line %q in\n%s", want, text)
			}
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("missing line %q in\n%s", want, text)
	}
	return at
}

func TestAddGroupsWithDirectory(t *testing.T) {
	for _, crlf := range []bool{false, true} {
		text := sample
		if crlf {
			text = strings.ReplaceAll(text, "\n", "\r\n")
		}
		got := string(Add([]byte(text), "voidcrew/mapping/planet_projects/glasswood.dm"))
		at := index(t, got, `#include "voidcrew\mapping\planet_projects\glasswood.dm"`)
		if index(t, got, `#include "voidcrew\mapping\docking_port\_docking_port.dm"`) != at-1 || index(t, got, `#include "voidcrew\mapping\shuttles\_shuttle.dm"`) != at+1 {
			t.Fatalf("planet registered outside its directory group:\n%s", got)
		}
		wantEndings := strings.Count(text, "\r\n")
		if crlf {
			wantEndings++
		}
		if strings.Count(got, "\r\n") != wantEndings {
			t.Fatal("line endings changed")
		}
		if len(lines(got)) != len(lines(text))+1 {
			t.Fatalf("unrelated lines changed:\n%s", got)
		}
		if again := string(Add([]byte(got), "voidcrew\\mapping\\planet_projects\\Glasswood.dm")); again != got {
			t.Fatalf("second registration was not idempotent:\n%s", again)
		}
	}
}

func TestAddOrdersLikeDreamMaker(t *testing.T) {
	steps := []struct{ relative, previous, next string }{
		// A directory's own files come before its child folders.
		{"voidcrew/mapping/zones.dm", `#include "voidcrew\mapping\rivers.dm"`, `#include "voidcrew\mapping\docking_port\_docking_port.dm"`},
		// Alphabetical among sibling files.
		{"voidcrew/mapping/shuttles/carp.dm", `#include "voidcrew\mapping\shuttles\box.dm"`, `#include "voidcrew\mapping\shuttles\delta.dm"`},
		// A new child folder sorts among the existing folders of the nearest group.
		{"voidcrew/mapping/ruins/beach/wreck.dm", `#include "voidcrew\mapping\docking_port\_docking_port.dm"`, `#include "voidcrew\mapping\shuttles\_shuttle.dm"`},
		{"voidcrew/mapping/ship_crew/delta.dm", `#include "voidcrew\mapping\ruins\beach\wreck.dm"`, `#include "voidcrew\mapping\shuttles\_shuttle.dm"`},
		// A file after every sibling lands at the end of the group.
		{"voidcrew/mapping/spawners/z.dm", `#include "voidcrew\mapping\spawners\nanites.dm"`, `#include "voidcrew\modules\ship_upgrades\_ship_upgrades.dm"`},
		// A new folder joins its parent's group even when that group is elsewhere.
		{"voidcrew/modules/ship_upgrades/ships/test.dm", `#include "voidcrew\modules\ship_upgrades\admin_verbs.dm"`, `#include "voidcrew\turfs\open\planet.dm"`},
	}
	text := sample
	for _, step := range steps {
		text = string(Add([]byte(text), step.relative))
		at := index(t, text, `#include "`+strings.ReplaceAll(step.relative, "/", "\\")+`"`)
		if index(t, text, step.previous) != at-1 || index(t, text, step.next) != at+1 {
			t.Fatalf("%s is out of order:\n%s", step.relative, text)
		}
	}
	if len(lines(text)) != len(lines(sample))+len(steps) {
		t.Fatalf("unrelated lines changed:\n%s", text)
	}
}

func TestAddMovesLegacyRegistrationIntoGroup(t *testing.T) {
	legacy := strings.Replace(sample, "// END_INCLUDE\n", `#include "voidcrew/mapping/shuttles/test.dm"
#include "voidcrew/modules/ship_upgrades/ships/test.dm"
#include "voidcrew\mapping\ruins\beach\test.dm"
#include "voidcrew/mapping/ship_crew/delta.dm" // keep me
// END_INCLUDE
`, 1)
	got := string(Add([]byte(legacy), "voidcrew/mapping/ship_crew/delta.dm"))
	at := index(t, got, `#include "voidcrew\mapping\ship_crew\delta.dm" // keep me`)
	if index(t, got, `#include "voidcrew\mapping\docking_port\_docking_port.dm"`) != at-1 || index(t, got, `#include "voidcrew\mapping\shuttles\_shuttle.dm"`) != at+1 {
		t.Fatalf("legacy registration was not moved beside its directory:\n%s", got)
	}
	for _, stray := range []string{`#include "voidcrew/mapping/shuttles/test.dm"`, `#include "voidcrew/modules/ship_upgrades/ships/test.dm"`, `#include "voidcrew\mapping\ruins\beach\test.dm"`} {
		index(t, got, stray)
	}
	if !strings.HasSuffix(got, "#include \"voidcrew\\mapping\\ruins\\beach\\test.dm\"\n// END_INCLUDE\n") || len(lines(got)) != len(lines(legacy)) {
		t.Fatalf("other legacy registrations were reordered:\n%s", got)
	}
	// A sibling of a remaining legacy entry joins it instead of the parent group.
	got = string(Add([]byte(got), "voidcrew/modules/ship_upgrades/ships/carp.dm"))
	at = index(t, got, `#include "voidcrew\modules\ship_upgrades\ships\carp.dm"`)
	if index(t, got, `#include "voidcrew/modules/ship_upgrades/ships/test.dm"`) != at+1 {
		t.Fatalf("new registration ignored its legacy sibling:\n%s", got)
	}
}

func TestAddWithoutGroupsOrMarker(t *testing.T) {
	if got := string(Add(nil, "voidcrew/mapping/planet_projects/a.dm")); got != "#include \"voidcrew\\mapping\\planet_projects\\a.dm\"\n" {
		t.Fatalf("empty project: %q", got)
	}
	got := string(Add([]byte("// BEGIN_INCLUDE\r\n// END_INCLUDE\r\n"), "code/a.dm"))
	if got != "// BEGIN_INCLUDE\r\n#include \"code\\a.dm\"\r\n// END_INCLUDE\r\n" {
		t.Fatalf("marker only: %q", got)
	}
	got = string(Add([]byte(`#include "content.dm"`), "voidcrew/mapping/planet_projects/a.dm"))
	if got != "#include \"content.dm\"\n#include \"voidcrew\\mapping\\planet_projects\\a.dm\"\n" {
		t.Fatalf("missing final newline: %q", got)
	}
	got = string(Add([]byte("#include \"z\\b.dm\"\n#include \"z\\c.dm\"\n\n#define LATE\n"), "z/a.dm"))
	if got != "#include \"z\\a.dm\"\n#include \"z\\b.dm\"\n#include \"z\\c.dm\"\n\n#define LATE\n" {
		t.Fatalf("group without marker: %q", got)
	}
	for _, bad := range []string{"", ".", "../escape.dm", "/abs.dm"} {
		if got := string(Add([]byte(sample), bad)); got != sample {
			t.Fatalf("registered invalid path %q", bad)
		}
	}
}

func TestAddRespectsCommentsAndConditionals(t *testing.T) {
	text := `/* Editor notes:
// END_INCLUDE
#include "code\b.dm" (planned)
*/
#include "code\a.dm"
#ifdef UNIT_TESTS
#include "code/tests/c.dm" // conditional
#endif
#include "code\d.dm" // #include "code\e.dm"
// END_INCLUDE
`
	got := string(Add([]byte(text), "code/b.dm"))
	if index(t, got, `#include "code\b.dm"`) != index(t, got, `#include "code\a.dm"`)+1 {
		t.Fatalf("commented include or fake marker changed placement:\n%s", got)
	}
	got = string(Add([]byte(got), "code/e.dm"))
	if index(t, got, `#include "code\e.dm"`) != index(t, got, `#include "code\b.dm"`)+1 {
		t.Fatalf("include inside a trailing comment counted as registered:\n%s", got)
	}
	got = string(Add([]byte(got), "code/tests/c.dm"))
	if strings.Count(got, `"code\tests\c.dm"`) != 1 || !strings.Contains(got, "#ifdef UNIT_TESTS\n#include \"code\\tests\\c.dm\" // conditional\n#endif") {
		t.Fatalf("conditional registration left its scope or was duplicated:\n%s", got)
	}
	if !strings.HasPrefix(got, "/* Editor notes:\n// END_INCLUDE\n#include \"code\\b.dm\" (planned)\n*/\n") || !strings.HasSuffix(got, "#include \"code\\d.dm\" // #include \"code\\e.dm\"\n// END_INCLUDE\n") {
		t.Fatalf("comments were rewritten:\n%s", got)
	}
}

func TestAddNormalizesDuplicates(t *testing.T) {
	text := "#include \"Code/A.dm\" // first\n#include \"code\\b.dm\"\n#include \"code\\a.dm\"\n"
	got := string(Add([]byte(text), "code/a.dm"))
	if got != "#include \"Code\\A.dm\" // first\n#include \"code\\b.dm\"\n" {
		t.Fatalf("duplicates not collapsed onto the first registration: %q", got)
	}
}
