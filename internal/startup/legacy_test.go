package startup

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLegacyShortcutInstallsOnceAndFollowsCurrentExecutable(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows executable migration")
	}
	folder := t.TempDir()
	legacy := filepath.Join(folder, "StrongDMM.exe")
	if err := os.WriteFile(legacy, []byte("bridge build"), 0700); err != nil {
		t.Fatal(err)
	}
	executable, err := legacyExecutable(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(executable) != "Voidworks.exe" {
		t.Fatal(executable)
	}
	if got, _ := os.ReadFile(executable); string(got) != "bridge build" {
		t.Fatal("incomplete migration")
	}
	if err := os.WriteFile(executable, []byte("newer beta build"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := legacyExecutable(legacy); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(executable); string(got) != "newer beta build" {
		t.Fatal("old shortcut replaced a newer build")
	}
	if got, _ := os.ReadFile(legacy); string(got) != "bridge build" {
		t.Fatal("removed old shortcut target")
	}
}

func TestLegacyCleanupEnvironmentIsForwardedThenRemoved(t *testing.T) {
	t.Setenv(cleanupEnvironment, "")
	t.Setenv("STRONGDMM_UPDATE_CLEANUP", "legacy staging folder")
	if cleanupDirectory() != "legacy staging folder" {
		t.Fatal("lost old supervisor cleanup")
	}
	for _, value := range updateEnvironment() {
		if value == "STRONGDMM_UPDATE_CLEANUP=legacy staging folder" {
			t.Fatal("leaked old supervisor environment")
		}
	}
}
