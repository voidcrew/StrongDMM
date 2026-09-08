package selfupdate

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"debug/pe"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const stagePrefix = ".strongdmm-update-"
const maxExpanded = 512 << 20

// The editor goes last, so failed companion-file replacements never install a
// new executable. User projects and profile data are outside this allowlist.
var packageFiles = []string{"LICENSE", "START-HERE.txt", "BUILD-INFO.json", "Source.zip", "SHA256SUMS.txt", "shipcheck.exe", "StrongDMM.exe"}

type Staged struct {
	Directory string
	Release   Release
}

func regularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("expected a regular file: %s", path)
	}
	return nil
}

func installDirectory(executable string) (string, error) {
	if !strings.EqualFold(filepath.Base(executable), "StrongDMM.exe") {
		return "", fmt.Errorf("run StrongDMM.exe from an extracted Windows package to update")
	}
	if err := regularFile(executable); err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(filepath.Dir(executable))
}

// Stage downloads and validates a complete release without changing the running
// installation. Keeping it on the same volume makes replacement renames atomic.
func Stage(ctx context.Context, release Release, executable string) (*Staged, error) {
	return stage(ctx, httpClient(), release, executable)
}

func stage(ctx context.Context, client *http.Client, release Release, executable string) (_ *Staged, err error) {
	dir, err := installDirectory(executable)
	if err != nil {
		return nil, err
	}
	folder, err := os.MkdirTemp(dir, stagePrefix)
	if err != nil {
		return nil, fmt.Errorf("the installation folder must be writable; extract StrongDMM to a folder you own: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(folder)
		}
	}()
	staged := &Staged{Directory: folder, Release: release}
	resp, err := get(ctx, client, release.URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	file, err := os.OpenFile(filepath.Join(folder, "package.zip"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	n, copyErr := io.Copy(file, io.LimitReader(resp.Body, maxDownload+1))
	closeErr := file.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("download update: %w", copyErr)
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if n > maxDownload || n != release.Size {
		return nil, fmt.Errorf("downloaded package is incomplete or has an unexpected size")
	}
	verified, err := staged.unpack()
	if err != nil {
		return nil, err
	}
	_ = os.RemoveAll(verified) // Installation extracts again from the pinned archive.
	return staged, nil
}

func fileHash(path string) (string, error) {
	if err := regularFile(path); err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Staged) unpack() (string, error) {
	archive := filepath.Join(s.Directory, "package.zip")
	info, err := os.Stat(archive)
	if err != nil {
		return "", err
	}
	if info.Size() != s.Release.Size || info.Size() > maxDownload {
		return "", fmt.Errorf("update archive size changed")
	}
	digest, err := fileHash(archive)
	if err != nil {
		return "", err
	}
	if digest != s.Release.SHA256 {
		return "", fmt.Errorf("update checksum does not match GitHub; download the update again")
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return "", fmt.Errorf("open update package: %w", err)
	}
	defer reader.Close()
	if len(reader.File) != len(packageFiles) {
		return "", fmt.Errorf("unexpected update package contents")
	}
	extracted, err := os.MkdirTemp(s.Directory, "verified-")
	if err != nil {
		return "", err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(extracted)
		}
	}()
	remaining := map[string]bool{}
	for _, name := range packageFiles {
		remaining[packageName(s.Release.Version)+"/"+name] = true
	}
	var total uint64
	for _, entry := range reader.File {
		if !remaining[entry.Name] || !entry.Mode().IsRegular() {
			return "", fmt.Errorf("unsafe or duplicate package entry: %s", entry.Name)
		}
		delete(remaining, entry.Name)
		if entry.UncompressedSize64 > maxExpanded {
			return "", fmt.Errorf("update package is too large")
		}
		total += entry.UncompressedSize64
		if total > maxExpanded {
			return "", fmt.Errorf("expanded update package is too large")
		}
		if err := extractFile(entry, filepath.Join(extracted, filepath.Base(entry.Name))); err != nil {
			return "", err
		}
	}
	if err := verifyContents(extracted, s.Release.Version); err != nil {
		return "", err
	}
	success = true
	return extracted, nil
}

func extractFile(entry *zip.File, path string) error {
	input, err := entry.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(output, io.LimitReader(input, int64(entry.UncompressedSize64)+1))
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if uint64(n) != entry.UncompressedSize64 {
		return fmt.Errorf("invalid size for %s", entry.Name)
	}
	return nil
}

func verifyContents(folder, version string) error {
	data, err := os.ReadFile(filepath.Join(folder, "SHA256SUMS.txt"))
	if err != nil {
		return err
	}
	checksums := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.Split(strings.TrimSpace(line), "  ")
		if len(parts) != 2 || len(parts[0]) != 64 || checksums[parts[1]] != "" {
			return fmt.Errorf("invalid package checksums")
		}
		checksums[parts[1]] = parts[0]
	}
	if len(checksums) != len(packageFiles)-1 {
		return fmt.Errorf("missing package checksums")
	}
	for _, name := range packageFiles {
		if name == "SHA256SUMS.txt" {
			continue
		}
		digest, err := fileHash(filepath.Join(folder, name))
		if err != nil {
			return err
		}
		if digest != checksums[name] {
			return fmt.Errorf("package checksum mismatch: %s", name)
		}
	}
	data, err = os.ReadFile(filepath.Join(folder, "BUILD-INFO.json"))
	if err != nil {
		return err
	}
	var build struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &build); err != nil || build.Version != version {
		return fmt.Errorf("package version does not match the release")
	}
	for _, name := range []string{"StrongDMM.exe", "shipcheck.exe"} {
		binary, err := pe.Open(filepath.Join(folder, name))
		if err != nil {
			return fmt.Errorf("invalid Windows executable %s: %w", name, err)
		}
		machine := binary.Machine
		_, pe64 := binary.OptionalHeader.(*pe.OptionalHeader64)
		binary.Close()
		if machine != pe.IMAGE_FILE_MACHINE_AMD64 || !pe64 {
			return fmt.Errorf("%s is not a Windows x64 executable", name)
		}
	}
	return nil
}

func validateStage(folder, executable string) error {
	dir, err := installDirectory(executable)
	if err != nil {
		return err
	}
	info, err := os.Lstat(folder)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("invalid update staging folder")
	}
	resolved, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Dir(resolved), dir) || !strings.HasPrefix(filepath.Base(resolved), stagePrefix) {
		return fmt.Errorf("update staging folder is outside the installation")
	}
	return nil
}

func (s *Staged) Discard(executable string) error {
	if err := validateStage(s.Directory, executable); err != nil {
		return err
	}
	return os.RemoveAll(s.Directory)
}

type Installation struct {
	folder    string
	backup    string
	installed []string
	originals []string
}

// Install runs in the supervisor after the editor has saved and exited. Windows
// permits renaming the supervisor's running image; its backup is removed only by
// the next process after this supervisor exits.
func Install(s Staged, executable string) (*Installation, error) {
	return install(s, executable, os.Rename)
}

func install(s Staged, executable string, rename func(string, string) error) (*Installation, error) {
	if err := validateStage(s.Directory, executable); err != nil {
		return nil, err
	}
	verified, err := s.unpack() // Recheck the pinned archive; never trust stale extracted files.
	if err != nil {
		return nil, err
	}
	folder := filepath.Dir(executable)
	if data, err := os.ReadFile(filepath.Join(folder, "BUILD-INFO.json")); err == nil {
		var current struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &current); err != nil {
			return nil, fmt.Errorf("read installed build information: %w", err)
		}
		if !Newer(s.Release.Version, current.Version) {
			return nil, fmt.Errorf("this installation already has version %s or newer", current.Version)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for _, name := range packageFiles {
		if err := regularFile(filepath.Join(folder, name)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	backup, err := os.MkdirTemp(s.Directory, "previous-")
	if err != nil {
		return nil, err
	}
	change := &Installation{folder: folder, backup: backup}
	for _, name := range packageFiles {
		dest := filepath.Join(folder, name)
		if _, err = os.Lstat(dest); err == nil {
			err = rename(dest, filepath.Join(backup, name))
			if err == nil {
				change.originals = append(change.originals, name)
			}
		} else if os.IsNotExist(err) {
			err = nil
		}
		if err == nil {
			err = rename(filepath.Join(verified, name), dest)
			if err == nil {
				change.installed = append(change.installed, name)
			}
		}
		if err != nil {
			return nil, errors.Join(fmt.Errorf("replace %s: %w", name, err), change.Rollback())
		}
	}
	return change, nil
}

func (c *Installation) Rollback() error {
	var failures []error
	for i := len(c.installed) - 1; i >= 0; i-- {
		if err := os.Remove(filepath.Join(c.folder, c.installed[i])); err != nil {
			failures = append(failures, err)
		}
	}
	for i := len(c.originals) - 1; i >= 0; i-- {
		name := c.originals[i]
		if err := os.Rename(filepath.Join(c.backup, name), filepath.Join(c.folder, name)); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("restore previous build from %s: %w", c.backup, errors.Join(failures...))
	}
	return nil
}
