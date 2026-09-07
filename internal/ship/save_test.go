package ship

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveConflictTouchesNoDestination(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	_ = os.WriteFile(a, []byte("original a"), 0600)
	_ = os.WriteFile(b, []byte("external b"), 0600)
	changes := []FileChange{{Path: a, Before: []byte("original a"), After: []byte("edited a"), Existed: true}, {Path: b, Before: []byte("original b"), After: []byte("edited b"), Existed: true}}
	if WriteChanges(root, changes) == nil {
		t.Fatal("external edit accepted")
	}
	got, _ := os.ReadFile(a)
	if string(got) != "original a" {
		t.Fatal("first destination changed")
	}
	got, _ = os.ReadFile(b)
	if string(got) != "external b" {
		t.Fatal("external edit overwritten")
	}
}
func TestSaveFailureRollsBackEveryReplacement(t *testing.T) {
	root := t.TempDir()
	var changes []FileChange
	for _, name := range []string{"a", "b", "c"} {
		path := filepath.Join(root, name)
		_ = os.WriteFile(path, []byte(name), 0600)
		changes = append(changes, FileChange{Path: path, Before: []byte(name), After: []byte("new " + name), Existed: true})
	}
	count := 0
	err := writeChanges(root, changes, func(from, to string) error {
		count++
		if count == 4 {
			return errors.New("injected rename failure")
		}
		return os.Rename(from, to)
	})
	if err == nil {
		t.Fatal("failure ignored")
	}
	for _, c := range changes {
		got, _ := os.ReadFile(c.Path)
		if !bytes.Equal(got, c.Before) {
			t.Fatalf("not restored: %s", c.Path)
		}
	}
	files, _ := os.ReadDir(root)
	if len(files) != 3 {
		t.Fatalf("temporary files left: %v", files)
	}
}
func TestSaveNewFileCollisionAndEscape(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "existing")
	_ = os.WriteFile(path, []byte("keep"), 0600)
	if WriteChanges(root, []FileChange{{Path: path, After: []byte("replace")}}) == nil {
		t.Fatal("new file overwrote existing")
	}
	if WriteChanges(root, []FileChange{{Path: filepath.Join(root, "..", "outside"), After: []byte("bad")}}) == nil {
		t.Fatal("path escaped")
	}
}
