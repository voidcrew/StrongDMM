package startup

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"sdmm/internal/app/selfupdate"
	"sdmm/internal/env"
)

const updateTestEnvironment = "STRONGDMM_SUPERVISOR_UPDATE_TEST"

// This helper runs the production supervisor/editor protocol from a copied,
// running Windows executable. Its editor callback avoids creating an OpenGL UI.
func TestMain(m *testing.M) {
	if folder := os.Getenv(updateTestEnvironment); folder != "" {
		data, err := os.ReadFile(filepath.Join(folder, "BUILD-INFO.json"))
		if err != nil {
			panic(err)
		}
		var info struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &info); err != nil {
			panic(err)
		}
		env.Version = info.Version
		os.Exit(Run(func() {
			if info.Version == "0.5.2" {
				data, _ := json.Marshal(os.Args[1:])
				if err := os.WriteFile(filepath.Join(folder, "restarted.json"), data, 0600); err != nil {
					panic(err)
				}
				// Give the supervisor's cleanup worker time to remove the old image.
				deadline := time.Now().Add(10 * time.Second)
				for time.Now().Before(deadline) {
					if _, err := os.Stat(filepath.Join(folder, ".strongdmm-update-fixture")); os.IsNotExist(err) {
						return
					}
					time.Sleep(50 * time.Millisecond)
				}
				panic("old executable staging folder was not cleaned")
			}
			data, err := os.ReadFile(filepath.Join(folder, "staged.json"))
			if err != nil {
				panic(err)
			}
			var staged selfupdate.Staged
			if err := json.Unmarshal(data, &staged); err != nil {
				panic(err)
			}
			if err := ScheduleUpdate(staged, []string{filepath.Join(folder, "project & [test] ü.dme"), "--ship-workspace"}); err != nil {
				panic(err)
			}
		}))
	}
	os.Exit(m.Run())
}

func TestWindowsUpdateAndRestart(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows supervisor")
	}
	folder := filepath.Join(t.TempDir(), "StrongDMM update & [test]")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(folder, "StrongDMM.exe")
	if err := os.WriteFile(exe, binary, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "BUILD-INFO.json"), []byte(`{"version":"0.5.1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"StrongDMM.exe": binary, "shipcheck.exe": binary, "LICENSE": []byte("license"),
		"Source.zip": []byte("source"), "START-HERE.txt": []byte("instructions"), "BUILD-INFO.json": []byte(`{"version":"0.5.2"}`),
	}
	var checksums strings.Builder
	for name, data := range files {
		fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(data), name)
	}
	files["SHA256SUMS.txt"] = []byte(checksums.String())
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, data := range files {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: "StrongDMM-Voidcrew-0.5.2-windows-x64/" + name, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	stageDir := filepath.Join(folder, ".strongdmm-update-fixture")
	if err := os.Mkdir(stageDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stageDir, "package.zip"), archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	staged := selfupdate.Staged{Directory: stageDir, Release: selfupdate.Release{Version: "0.5.2", SHA256: fmt.Sprintf("%x", sha256.Sum256(archive.Bytes())), Size: int64(archive.Len())}}
	data, _ := json.Marshal(staged)
	if err := os.WriteFile(filepath.Join(folder, "staged.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	configureProcess(cmd)
	profile := filepath.Join(folder, "profile")
	cmd.Env = append(updateEnvironment(), updateTestEnvironment+"="+folder, "APPDATA="+profile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("old supervisor: %v\n%s", err, output)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(folder, "restarted.json"))
		_, stageErr := os.Stat(stageDir)
		if err == nil && os.IsNotExist(stageErr) {
			var args []string
			if err := json.Unmarshal(data, &args); err != nil {
				t.Fatal(err)
			}
			if len(args) != 2 || args[0] != filepath.Join(folder, "project & [test] ü.dme") || args[1] != "--ship-workspace" {
				t.Fatalf("restart arguments changed: %q", args)
			}
			// Wait until the replacement supervisor has released its image, too.
			for time.Now().Before(deadline) {
				if err := os.Remove(exe); err == nil {
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("replacement editor did not finish or old image was not cleaned")
}

func TestRestartNeedsSupervisor(t *testing.T) {
	t.Setenv(requestEnvironment, "")
	if err := ScheduleUpdate(selfupdate.Staged{}, nil); err == nil {
		t.Fatal("scheduled unsupervised update")
	}
}

func TestRequestWrittenOnlyOnce(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows updater")
	}
	path := filepath.Join(t.TempDir(), "request.json")
	t.Setenv(requestEnvironment, path)
	defer func() { updateScheduled = false }()
	if err := ScheduleUpdate(selfupdate.Staged{}, []string{"project.dme"}); err != nil {
		t.Fatal(err)
	}
	if err := ScheduleUpdate(selfupdate.Staged{}, nil); err == nil {
		t.Fatal("overwrote pending restart")
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "project.dme") {
		t.Fatalf("lost original request: %v", err)
	}
}
