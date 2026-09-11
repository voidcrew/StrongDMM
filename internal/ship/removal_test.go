package ship

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sdmm/internal/dmapi/dmenv"
	"sdmm/internal/util"
)

func TestRemovalLivePreview(t *testing.T) {
	path := os.Getenv("SHIP_REMOVAL_TEST_DME")
	if path == "" {
		t.Skip("set SHIP_REMOVAL_TEST_DME for read-only project removal checks")
	}
	dme, err := dmenv.New(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Discover(dme)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range c.Hulls {
		if !strings.HasSuffix(h.Type, "/delta") && !strings.HasSuffix(h.Type, "/scarab") {
			continue
		}
		start := time.Now()
		plan, err := c.Removal(dme, h, false)
		if err != nil {
			t.Errorf("%s: %v", h.Name, err)
			continue
		}
		t.Logf("%s: %d changes, %d kept, checked in %s", h.Name, len(plan.Changes), len(plan.Kept), time.Since(start))
	}
}

func TestRemoveAuthoredShipAndRecovery(t *testing.T) {
	c, dme := authorEnvironment(t)
	p, err := NewProject(c, dme, "remove_me", "Remove me", 12, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.addRect(0, "room", "Room", util.Point{X: 3, Y: 3, Z: 1}, util.Point{X: 5, Y: 5, Z: 1}); err != nil {
		t.Fatal(err)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	c.Hulls = append(c.Hulls, p.Hull)
	plan, err := c.Removal(dme, p.Hull, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) < 5 {
		t.Fatalf("incomplete removal: %+v", plan.Changes)
	}
	before := map[string][]byte{}
	for _, change := range plan.Changes {
		before[change.Path] = append([]byte{}, change.Before...)
	}
	backup, err := plan.Apply(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range plan.Changes {
		data, err := os.ReadFile(change.Path)
		if change.Delete {
			if !os.IsNotExist(err) {
				t.Fatalf("not deleted: %s", change.Path)
			}
		} else if err != nil || !bytes.Equal(data, change.After) {
			t.Fatalf("include not updated: %s", change.Path)
		}
	}
	archive, err := zip.OpenReader(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != len(before)+1 {
		t.Fatal("incomplete recovery archive")
	}
	for _, entry := range archive.File {
		if entry.Name == "REMOVAL.json" {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		var data bytes.Buffer
		_, err = data.ReadFrom(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data.Bytes(), before[filepath.Join(c.Root, filepath.FromSlash(entry.Name))]) {
			t.Fatal("recovery copy changed", entry.Name)
		}
	}
	dme.RemoveTypeTrees(plan.Types)
	c.Hulls = nil
	if _, err := NewProject(c, dme, "remove_me", "Remove me", 12, 12); err != nil {
		t.Fatalf("removed ship identifier cannot be reused: %v", err)
	}
}

func TestRemoveSavedBlankShipAndUndoneModules(t *testing.T) {
	for _, withRoom := range []bool{false, true} {
		c, dme := authorEnvironment(t)
		p, err := NewProject(c, dme, "remove_me", "Remove me", 12, 12)
		if err != nil {
			t.Fatal(err)
		}
		initial := p.Capture()
		if withRoom {
			if err := p.addRect(0, "room", "Room", util.Point{X: 3, Y: 3, Z: 1}, util.Point{X: 5, Y: 5, Z: 1}); err != nil {
				t.Fatal(err)
			}
		}
		if err := p.Save(); err != nil {
			t.Fatal(err)
		}
		p.Restore(initial)
		c.Hulls = []Hull{p.Hull}
		plan, err := c.Removal(dme, p.Hull, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plan.Apply(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		dme.RemoveTypeTrees(plan.Types)
		c.Hulls = nil
		if _, err := NewProject(c, dme, "remove_me", "Remove me", 12, 12); err != nil {
			t.Fatal("cannot reuse removed identifier", err)
		}
		if withRoom {
			files, err := projectMapFiles(c.Root)
			if err != nil || len(files) != 0 {
				t.Fatal("saved modules survived unsaved undo", files, err)
			}
		}
	}
}

func TestRemovalCannotEditExternalIncludedSources(t *testing.T) {
	_, dme := authorEnvironment(t)
	external := filepath.Join(t.TempDir(), "external.dm")
	original := []byte(HullType + "/outside\n\tname = \"Outside\"\n")
	if err := os.WriteFile(external, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dme.RootFile, []byte("#include \""+filepath.ToSlash(external)+"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanRemoval(dme, RemovalSpec{Name: "Outside", Types: []string{HullType + "/outside"}}); err == nil {
		t.Fatal("allowed external source removal")
	}
	data, _ := os.ReadFile(external)
	if !bytes.Equal(data, original) {
		t.Fatal("external project changed")
	}
}

func TestRemovalKeepsNeighborCommentsAndMarkers(t *testing.T) {
	before := []byte("/datum/example/remove\n\tname = \"remove\"\n\n// BEGIN RUIN WORKSHOP: /datum/example/keep\n/datum/example/keep\n\tname = \"keep\"\n// END RUIN WORKSHOP: /datum/example/keep\n")
	after, _, err := removeDefinitions(before, []string{"/datum/example/remove"})
	if err != nil || !bytes.Contains(after, []byte("// BEGIN RUIN WORKSHOP: /datum/example/keep")) {
		t.Fatal("lost neighbor marker", err)
	}
}

func TestRemovalPreservesSharedMapsAndHelpers(t *testing.T) {
	c, dme := authorEnvironment(t)
	p, err := NewProject(c, dme, "shared_ship", "Shared ship", 12, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	other := p.Hull
	other.Type = HullType + "/other_ship"
	c.Hulls = []Hull{p.Hull, other}
	plan, err := c.Removal(dme, p.Hull, false)
	if err != nil {
		t.Fatal(err)
	}
	file, _ := c.HullFile(p.Hull, p.Hull.Themes[0])
	for _, change := range plan.Changes {
		if change.Delete && change.Path == file {
			t.Fatal("deleted shared hull")
		}
	}
	if len(plan.Kept) == 0 || plan.RemovesType(p.areaType()) || plan.RemovesType(p.portType()) {
		t.Fatal("shared map lost its area/port types")
	}
	if _, err := plan.Apply(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("shared map lost")
	}
	data, err := os.ReadFile(filepath.Join(c.Root, "voidcrew/mapping/shuttles/shared_ship.dm"))
	if err != nil || !bytes.Contains(data, []byte(p.areaType())) || bytes.Contains(data, []byte(p.Hull.Type)) {
		t.Fatal("shared definitions were not preserved correctly", err)
	}
}

func TestRemovalRejectsStalePlanAndReferences(t *testing.T) {
	for _, mode := range []string{"external edit", "new map", "code dependency", "map dependency"} {
		t.Run(mode, func(t *testing.T) {
			c, dme := authorEnvironment(t)
			p, err := NewProject(c, dme, "remove_me", "Remove me", 12, 12)
			if err != nil {
				t.Fatal(err)
			}
			if err := p.Save(); err != nil {
				t.Fatal(err)
			}
			c.Hulls = []Hull{p.Hull}
			if mode == "code dependency" {
				f, err := os.OpenFile(dme.RootFile, os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = f.WriteString("\n/datum/other\n\tvar/template = " + p.Hull.Type + "\n")
				f.Close()
			}
			if mode == "map dependency" {
				if err := os.WriteFile(filepath.Join(c.Root, "_maps/other.dmm"), []byte("some map using "+p.Hull.Type), 0600); err != nil {
					t.Fatal(err)
				}
			}
			plan, err := c.Removal(dme, p.Hull, false)
			if strings.Contains(mode, "dependency") {
				if err == nil {
					t.Fatal("ignored live reference")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "new map" {
				err = os.WriteFile(filepath.Join(c.Root, "_maps/other.dmm"), []byte("new map"), 0600)
			} else {
				err = os.WriteFile(dme.RootFile, []byte("external edit"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := plan.Apply(t.TempDir()); err == nil {
				t.Fatal("accepted stale removal")
			}
			file, _ := c.HullFile(p.Hull, p.Hull.Themes[0])
			if _, err := os.Stat(file); err != nil {
				t.Fatal("changed map before conflict check")
			}
		})
	}
}

func TestMixedRemovalRollback(t *testing.T) {
	for _, failAt := range []int{2, 3, 4} {
		root := t.TempDir()
		var changes []FileChange
		for _, name := range []string{"a", "b", "c"} {
			file := filepath.Join(root, name)
			if err := os.WriteFile(file, []byte(name), 0600); err != nil {
				t.Fatal(err)
			}
			change := FileChange{Path: file, Before: []byte(name), Existed: true, Delete: name != "b"}
			if !change.Delete {
				change.After = []byte("new b")
			}
			changes = append(changes, change)
		}
		calls := 0
		err := writeChanges(root, changes, func(from, to string) error {
			calls++
			if calls == failAt {
				return errors.New("simulated lock")
			}
			return os.Rename(from, to)
		})
		if err == nil {
			t.Fatal("expected replacement failure")
		}
		for _, change := range changes {
			data, err := os.ReadFile(change.Path)
			if err != nil || !bytes.Equal(data, change.Before) {
				t.Fatal("rollback lost file", change.Path, err)
			}
		}
	}
}

func TestSourceRemovalKeepsNeighborAndConditionalStructure(t *testing.T) {
	before := []byte("#if ENABLED\r\n/datum/example/remove\r\n\tname = \"remove\"\r\n/datum/example/remove/proc/custom()\r\n\treturn 1\r\n#endif\r\n/datum/example/keep\r\n\tname = \"keep\"\r\n")
	after, _, err := removeDefinitions(before, []string{"/datum/example/remove"})
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != "#if ENABLED\r\n#endif\r\n/datum/example/keep\r\n\tname = \"keep\"\r\n" {
		t.Fatalf("damaged adjacent code: %s", after)
	}
	if _, _, err := removeDefinitions([]byte("/datum/example/remove\n\t#if FOO\n\tname = 1\n\t#endif\n"), []string{"/datum/example/remove"}); err == nil {
		t.Fatal("accepted conditional body")
	}
}
