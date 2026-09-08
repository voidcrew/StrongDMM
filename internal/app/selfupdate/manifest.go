package selfupdate

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sdmm/internal/env"
)

const releaseAPI = "https://api.github.com/repos/voidcrew/StrongDMM/releases/latest"
const releaseDownloads = "https://github.com/voidcrew/StrongDMM/releases/download/"
const maxDownload = 256 << 20

type Release struct {
	Version     string
	Description string
	URL         string
	SHA256      string
	Size        int64
}

type githubRelease struct {
	Tag        string `json:"tag_name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Digest string `json:"digest"`
		Size   int64  `json:"size"`
	} `json:"assets"`
}

func Supported() bool { return runtime.GOOS == "windows" && runtime.GOARCH == "amd64" }

// Only numbered stable builds participate in automatic updates. Development
// builds and prereleases must never silently replace a user's installation.
func versionNumbers(version string) ([3]uint64, error) {
	var result [3]uint64
	parts := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(parts) != 3 {
		return result, fmt.Errorf("invalid stable version %q", version)
	}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return result, fmt.Errorf("invalid stable version %q", version)
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return result, fmt.Errorf("invalid stable version %q", version)
			}
		}
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return result, err
		}
		result[i] = n
	}
	return result, nil
}

func Newer(candidate, current string) bool {
	next, err := versionNumbers(candidate)
	if err != nil {
		return false
	}
	previous, err := versionNumbers(current)
	if err != nil {
		return false
	}
	for i := range next {
		if next[i] != previous[i] {
			return next[i] > previous[i]
		}
	}
	return false
}

func packageName(version string) string { return "StrongDMM-Voidcrew-" + version + "-windows-x64" }

func parseRelease(data []byte, current string) (Release, error) {
	var source githubRelease
	if err := json.Unmarshal(data, &source); err != nil {
		return Release{}, fmt.Errorf("read release details: %w", err)
	}
	if source.Draft || source.Prerelease {
		return Release{}, fmt.Errorf("the latest release is not a stable build")
	}
	if _, err := versionNumbers(source.Tag); err != nil {
		return Release{}, err
	}
	if !Newer(source.Tag, current) {
		return Release{}, nil
	}
	version := strings.TrimPrefix(source.Tag, "v")
	name := packageName(version) + ".zip"
	var result Release
	for _, asset := range source.Assets {
		if asset.Name != name {
			continue
		}
		if result.Version != "" {
			return Release{}, fmt.Errorf("release contains duplicate Windows packages")
		}
		digest := strings.TrimPrefix(asset.Digest, "sha256:")
		hash, err := hex.DecodeString(digest)
		if err != nil || len(hash) != 32 || asset.Digest == digest {
			return Release{}, fmt.Errorf("release package has no valid SHA-256 digest")
		}
		if asset.URL != releaseDownloads+source.Tag+"/"+name {
			return Release{}, fmt.Errorf("release package is outside voidcrew/StrongDMM")
		}
		if asset.Size <= 0 || asset.Size > maxDownload {
			return Release{}, fmt.Errorf("release package size is invalid")
		}
		result = Release{Version: version, Description: source.Body, URL: asset.URL, SHA256: strings.ToLower(digest), Size: asset.Size}
	}
	if result.Version == "" {
		return Release{}, fmt.Errorf("the latest release has no Windows x64 package")
	}
	return result, nil
}

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many download redirects")
			}
			switch req.URL.Host {
			case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
				if req.URL.Scheme == "https" && req.URL.User == nil {
					return nil
				}
			}
			return fmt.Errorf("download redirected outside GitHub")
		},
	}
}

func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "StrongDMM-Voidcrew/"+env.Version)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to GitHub: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		if resp.StatusCode == 403 || resp.StatusCode == 429 {
			return nil, fmt.Errorf("GitHub temporarily limited update checks; try again later")
		}
		return nil, fmt.Errorf("GitHub returned HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

func Check(ctx context.Context, current string) (Release, error) {
	if !Supported() {
		return Release{}, fmt.Errorf("automatic updates require Windows x64")
	}
	if _, err := versionNumbers(current); err != nil {
		return Release{}, fmt.Errorf("development builds do not receive automatic updates")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	resp, err := get(ctx, httpClient(), releaseAPI)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return Release{}, err
	}
	if len(data) > 1<<20 {
		return Release{}, fmt.Errorf("release details are too large")
	}
	return parseRelease(data, current)
}
