// bundlepreviews prepares the portable Windows preview tools at build time.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed dependencies.json
var manifest []byte

type dependency struct {
	Name, Kind, URL, SHA256 string
}

func main() {
	cache := flag.String("cache", "dst/preview-downloads", "verified dependency cache (include in Source.zip for offline builds)")
	output := flag.String("output", "internal/shippreview/preview-runtime.zip", "embedded runtime archive")
	flag.Parse()
	if err := build(*cache, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func download(cache string, dep dependency) ([]byte, error) {
	path := filepath.Join(cache, dep.Name)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Println("Downloading " + dep.Name)
		client := &http.Client{Timeout: 3 * time.Minute}
		response, e := client.Get(dep.URL)
		if e != nil {
			return nil, e
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("download %s: %s", dep.Name, response.Status)
		}
		data, err = io.ReadAll(io.LimitReader(response.Body, (128<<20)+1))
		if len(data) > 128<<20 {
			return nil, fmt.Errorf("download too large: %s", dep.Name)
		}
	} else if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != dep.SHA256 {
		return nil, fmt.Errorf("checksum mismatch: %s", dep.Name)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		return nil, err
	}
	return data, nil
}

func build(cache, output string) error {
	if err := os.MkdirAll(cache, 0755); err != nil {
		return err
	}
	var dependencies []dependency
	if err := json.Unmarshal(manifest, &dependencies); err != nil {
		return err
	}
	files := map[string][]byte{"DEPENDENCIES.json": manifest}
	for _, dep := range dependencies {
		data, err := download(cache, dep)
		if err != nil {
			return err
		}
		switch dep.Kind {
		case "source":
			continue
		case "renderer":
			files["dmm-tools.exe"] = data
			continue
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return err
		}
		for _, entry := range archive.File {
			if entry.FileInfo().IsDir() {
				continue
			}
			name := entry.Name
			if !filepath.IsLocal(filepath.FromSlash(name)) || strings.ContainsAny(name, "\\:") || !entry.Mode().IsRegular() {
				return fmt.Errorf("invalid runtime archive entry: %s", name)
			}
			if dep.Kind == "renderer-source" {
				if strings.Count(name, "/") != 1 || filepath.Base(name) != "LICENSE" {
					continue
				}
				name = "LICENSE-dmm-tools.txt"
			} else if dep.Kind == "pillow" {
				name = "Lib/site-packages/" + name
			}
			input, err := entry.Open()
			if err != nil {
				return err
			}
			content, err := io.ReadAll(input)
			input.Close()
			if err != nil {
				return err
			}
			if _, duplicate := files[name]; duplicate {
				return fmt.Errorf("duplicate runtime file: %s", name)
			}
			files[name] = content
		}
	}
	// The embeddable interpreter ignores system Python, user packages and PATH.
	files["python313._pth"] = []byte("python313.zip\n.\nLib/site-packages\n")
	for _, name := range []string{"python.exe", "python313.dll", "python313.zip", "vcruntime140.dll", "dmm-tools.exe", "LICENSE.txt", "LICENSE-dmm-tools.txt", "Lib/site-packages/PIL/_imaging.cp313-win_amd64.pyd"} {
		if len(files[name]) == 0 {
			return fmt.Errorf("missing bundled dependency: %s", name)
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0644)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err = entry.Write(files[name]); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(output, buffer.Bytes(), 0600); err != nil {
		return err
	}
	fmt.Printf("Bundled %d preview runtime files (%d bytes).\n", len(files), buffer.Len())
	return nil
}
