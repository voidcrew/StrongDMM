package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileMigrationPreservesOriginalAndExistingSettings(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "StrongDMM-Voidcrew")
	files := map[string]string{"config/preferences.json": "saved preferences", "backup/map.dmm": "map backup", "layout.ini": "saved layout"}
	for name, data := range files {
		path := filepath.Join(legacy, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	destination, err := migrateProfile(root, "Voidworks", "StrongDMM-Voidcrew")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range files {
		for _, folder := range []string{legacy, destination} {
			got, err := os.ReadFile(filepath.Join(folder, name))
			if err != nil || string(got) != want {
				t.Fatalf("lost %s in %s: %v", name, folder, err)
			}
		}
	}
	settings := filepath.Join(destination, "config/preferences.json")
	if err := os.WriteFile(settings, []byte("new preferences"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateProfile(root, "Voidworks", "StrongDMM-Voidcrew"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(settings); string(got) != "new preferences" {
		t.Fatal("overwrote current profile")
	}
}

func TestProfileMigrationDoesNotReplaceAFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Voidworks")
	if err := os.WriteFile(path, []byte("user file"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateProfile(root, "Voidworks", "StrongDMM-Voidcrew"); err == nil {
		t.Fatal("accepted non-directory profile")
	}
	if got, _ := os.ReadFile(path); string(got) != "user file" {
		t.Fatal("modified existing file")
	}
}
