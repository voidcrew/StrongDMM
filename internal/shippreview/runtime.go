package shippreview

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var runtimeMu sync.Mutex

func Bundled() bool { return len(bundledRuntime) != 0 }

// Runtime files live outside the installation so updates can replace the editor
// while a preview worker finishes using its original interpreter and renderer.
func prepareRuntime(profile string, data []byte) (string, error) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("invalid bundled preview runtime: %w", err)
	}
	seen := map[string]bool{}
	for _, file := range archive.File {
		name := strings.ToLower(file.Name)
		if !filepath.IsLocal(filepath.FromSlash(name)) || strings.ContainsAny(name, "\\:") || !file.Mode().IsRegular() || seen[name] || name == "runtime-ready" {
			return "", fmt.Errorf("invalid preview runtime file: %s", file.Name)
		}
		seen[name] = true
	}
	cache := filepath.Join(profile, "preview-runtime")
	version := fmt.Sprintf("%x", sha256.Sum256(data))
	destination := filepath.Join(cache, version)
	valid := func(dir string) bool {
		ready, err := os.ReadFile(filepath.Join(dir, "runtime-ready"))
		if err != nil || string(ready) != version {
			return false
		}
		for _, file := range archive.File {
			info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(file.Name)))
			if err != nil || !info.Mode().IsRegular() || uint64(info.Size()) != file.UncompressedSize64 {
				return false
			}
		}
		return len(archive.File) > 0
	}
	// A repair gets its own directory rather than replacing files a live worker
	// may still have open. Subsequent requests reuse that complete installation.
	matches, _ := os.ReadDir(cache)
	for _, entry := range matches {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), version) {
			dir := filepath.Join(cache, entry.Name())
			if valid(dir) {
				return dir, nil
			}
		}
	}
	if err = os.MkdirAll(cache, 0700); err != nil {
		return "", err
	}
	temp, err := os.MkdirTemp(cache, version+"-")
	if err != nil {
		return "", err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(temp)
		}
	}()
	for _, file := range archive.File {
		name := filepath.FromSlash(file.Name)
		path := filepath.Join(temp, name)
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return "", err
		}
		if err = unpackRuntimeFile(file, path); err != nil {
			return "", err
		}
	}
	if err = os.WriteFile(filepath.Join(temp, "runtime-ready"), []byte(version), 0600); err != nil {
		return "", err
	}
	if err = os.Rename(temp, destination); err == nil {
		return destination, nil
	}
	// Another editor may have installed the same runtime while we extracted it.
	if valid(destination) {
		return destination, nil
	}
	keep = true
	return temp, nil
}

func unpackRuntimeFile(file *zip.File, path string) error {
	input, err := file.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (c *Client) previewCommand() (string, []string, []string, error) {
	if !Bundled() {
		path, args, err := python()
		return path, args, nil, err
	}
	dir, err := prepareRuntime(c.profile, bundledRuntime)
	if err != nil {
		return "", nil, nil, fmt.Errorf("prepare bundled preview tools: %w", err)
	}
	var environment []string
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if !strings.EqualFold(name, "DMM_TOOLS") && !strings.EqualFold(name, "PYTHONHOME") && !strings.EqualFold(name, "PYTHONPATH") {
			environment = append(environment, value)
		}
	}
	environment = append(environment, "DMM_TOOLS="+filepath.Join(dir, "dmm-tools.exe"))
	return filepath.Join(dir, "python.exe"), []string{"-I"}, environment, nil
}
