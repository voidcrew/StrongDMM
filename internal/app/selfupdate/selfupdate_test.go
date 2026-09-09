package selfupdate

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fixturePackage(t *testing.T, mutate func(map[string][]byte), entryName func(string) string) ([]byte, Release) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("fixture uses the Windows test executable")
	}
	binary, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"Voidworks.exe": binary, "shipcheck.exe": binary, "Source.zip": []byte("matching source"),
		"LICENSE": []byte("license"), "START-HERE.txt": []byte("instructions"), "BUILD-INFO.json": []byte(`{"version":"0.5.2"}`),
	}
	var checksums strings.Builder
	for name, data := range files {
		fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(data), name)
	}
	files["SHA256SUMS.txt"] = []byte(checksums.String())
	if mutate != nil {
		mutate(files)
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, name := range packageFiles {
		data, ok := files[name]
		if !ok {
			continue
		}
		entry := packageName("0.5.2") + "/" + name
		if entryName != nil {
			entry = entryName(entry)
		}
		writer, err := archive.CreateHeader(&zip.FileHeader{Name: entry, Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	data := buffer.Bytes()
	return data, Release{Version: "0.5.2", Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
}

func fixtureInstallation(t *testing.T) string {
	t.Helper()
	folder := filepath.Join(t.TempDir(), "Voidworks folder & [test]")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range packageFiles {
		data := []byte("previous " + name)
		if name == "BUILD-INFO.json" {
			data = []byte(`{"version":"0.5.1"}`)
		}
		if err := os.WriteFile(filepath.Join(folder, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(folder, "my-map.dmm"), []byte("user map"), 0600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(folder, "Voidworks.exe")
}

func fixtureStage(t *testing.T, exe string, data []byte, release Release) Staged {
	t.Helper()
	folder, err := os.MkdirTemp(filepath.Dir(exe), stagePrefix)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "package.zip"), data, 0600); err != nil {
		t.Fatal(err)
	}
	return Staged{Directory: folder, Release: release}
}

func TestVerifiedInstallAndRollback(t *testing.T) {
	data, release := fixturePackage(t, nil, nil)
	for _, failAt := range []int{0, 1, 6, 13, 14} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			exe := fixtureInstallation(t)
			staged := fixtureStage(t, exe, data, release)
			calls := 0
			rename := func(from, to string) error {
				calls++
				if calls == failAt {
					return fmt.Errorf("simulated file lock")
				}
				return os.Rename(from, to)
			}
			change, err := install(staged, exe, rename)
			if failAt == 0 {
				if err != nil {
					t.Fatal(err)
				}
				if err := verifyContents(filepath.Dir(exe), release.Version); err != nil {
					t.Fatal(err)
				}
				if _, err := Install(staged, exe); err == nil {
					t.Fatal("reinstalled same version")
				}
				if err := change.Rollback(); err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("expected replacement failure")
			}
			for _, name := range packageFiles {
				got, err := os.ReadFile(filepath.Join(filepath.Dir(exe), name))
				want := "previous " + name
				if name == "BUILD-INFO.json" {
					want = `{"version":"0.5.1"}`
				}
				if err != nil || string(got) != want {
					t.Fatalf("rollback did not restore %s: %v", name, err)
				}
			}
			if got, _ := os.ReadFile(filepath.Join(filepath.Dir(exe), "my-map.dmm")); string(got) != "user map" {
				t.Fatal("modified user file")
			}
		})
	}
}

func TestRejectUnsafePackages(t *testing.T) {
	for _, test := range []struct {
		name    string
		mutate  func(map[string][]byte)
		entry   func(string) string
		corrupt bool
	}{
		{name: "corrupt archive", corrupt: true},
		{name: "missing file", mutate: func(f map[string][]byte) { delete(f, "Source.zip") }},
		{name: "tampered executable", mutate: func(f map[string][]byte) { f["Voidworks.exe"] = []byte("bad") }},
		{name: "path traversal", entry: func(s string) string { return "../" + s }},
		{name: "absolute path", entry: func(s string) string { return "C:/" + s }},
		{name: "duplicates", entry: func(s string) string { return packageName("0.5.2") + "/LICENSE" }},
		{name: "wrong version", entry: func(s string) string { return strings.ReplaceAll(s, "0.5.2", "0.5.3") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, release := fixturePackage(t, test.mutate, test.entry)
			if test.corrupt {
				data = append([]byte(nil), data...)
				data[len(data)/2] ^= 1
			}
			exe := fixtureInstallation(t)
			staged := fixtureStage(t, exe, data, release)
			if _, err := Install(staged, exe); err == nil {
				t.Fatal("accepted unsafe package")
			}
			if got, _ := os.ReadFile(exe); string(got) != "previous Voidworks.exe" {
				t.Fatal("changed installation before validation")
			}
		})
	}
}

func TestDownloadFailuresAndCancellation(t *testing.T) {
	data, release := fixturePackage(t, nil, nil)
	for _, mode := range []string{"ok", "truncated", "checksum", "http error", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "http error" {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				if mode == "truncated" {
					_, _ = w.Write(data[:20])
					return
				}
				_, _ = w.Write(data)
			}))
			defer server.Close()
			candidate := release
			candidate.URL = server.URL
			if mode == "checksum" {
				candidate.SHA256 = strings.Repeat("0", 64)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			exe := fixtureInstallation(t)
			staged, err := stage(ctx, server.Client(), candidate, exe)
			if mode == "ok" {
				if err != nil {
					t.Fatal(err)
				}
				if got, _ := os.ReadFile(exe); string(got) != "previous Voidworks.exe" {
					t.Fatal("download changed running executable")
				}
				if err := staged.Discard(exe); err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("accepted failed download")
			}
			leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(exe), stagePrefix+"*"))
			if len(leftovers) != 0 {
				t.Fatalf("abandoned staging folders: %v", leftovers)
			}
		})
	}
}

func TestStageMustBeBesideExecutable(t *testing.T) {
	exe := fixtureInstallation(t)
	out := t.TempDir()
	staged := Staged{Directory: out}
	if err := staged.Discard(exe); err == nil {
		t.Fatal("deleted unrelated directory")
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

// Opt-in check against the public feed and package, without installing it.
func TestPublicReleaseDownload(t *testing.T) {
	if os.Getenv("VOIDWORKS_TEST_PUBLIC_UPDATE") == "" {
		t.Skip("set VOIDWORKS_TEST_PUBLIC_UPDATE for the live release check")
	}
	release, err := Check(context.Background(), "0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	exe := fixtureInstallation(t)
	staged, err := Stage(context.Background(), release, exe)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Discard(exe)
	data, _ := json.Marshal(release)
	t.Log(string(data))
}
