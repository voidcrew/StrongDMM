package app

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestStartupArguments(t *testing.T) {
	folder := t.TempDir()
	args := parseStartupArgs([]string{folder, "a.DMM", "b.dmm", "--ship-workspace"})
	if args.project != filepath.Join(folder, "tgstation.dme") || !args.shipWorkspace || !reflect.DeepEqual(args.maps, []string{"a.DMM", "b.dmm"}) {
		t.Fatalf("dropped project and maps: %+v", args)
	}
	args = parseStartupArgs([]string{"Project with spaces.DME"})
	if args.project != "Project with spaces.DME" || args.shipWorkspace || len(args.maps) != 0 {
		t.Fatalf("project launch should open the project home: %+v", args)
	}
}
