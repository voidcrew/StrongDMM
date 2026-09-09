package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func channelFixture(t *testing.T, tag string, prerelease bool) githubRelease {
	t.Helper()
	name := packageName(strings.TrimPrefix(tag, "v")) + ".zip"
	data := fmt.Sprintf(`{"tag_name":%q,"prerelease":%t,"assets":[{"name":%q,"browser_download_url":%q,"digest":%q,"size":100}]}`,
		tag, prerelease, name, releaseDownloads+tag+"/"+name, "sha256:"+strings.Repeat("a", 64))
	var release githubRelease
	if err := json.Unmarshal([]byte(data), &release); err != nil {
		t.Fatal(err)
	}
	return release
}

func TestBetaVersionOrdering(t *testing.T) {
	for _, test := range []struct {
		next, current string
		newer         bool
	}{
		{"0.5.14-beta.2", "0.5.14-beta.1", true},
		{"0.5.14-beta.10", "0.5.14-beta.2", true},
		{"0.5.14-beta.1", "0.5.14-beta.1", false},
		{"0.5.14-beta.1", "0.5.13-beta.9", true},
		{"0.5.14", "0.5.14-beta.9", true},
		{"0.5.14-beta.9", "0.5.14", false},
		{"0.5.14-beta.0", "0.5.13", false},
		{"0.5.14-beta.01", "0.5.13", false},
		{"0.5.14-beta.+1", "0.5.13", false},
		{"0.5.14-beta.1+local", "0.5.13", false},
		{"0.5.14-planet1", "0.5.13", false},
	} {
		if Newer(test.next, test.current) != test.newer {
			t.Errorf("ordering %q after %q", test.next, test.current)
		}
	}
}

func TestChannelSwitchPermission(t *testing.T) {
	for _, test := range []struct {
		next, current, from string
		allowed             bool
	}{
		{"0.5.14-beta.1", "0.5.13", "0.5.13", true},
		{"0.5.13", "0.5.14-beta.1", "0.5.14-beta.1", true},
		{"0.5.14-beta.1", "0.5.13", "", false},
		{"0.5.13", "0.5.14-beta.1", "", false},
		{"0.5.13", "0.5.14-beta.2", "0.5.14-beta.1", false},
		{"0.5.12", "0.5.13", "0.5.13", false},
		{"0.5.14-beta.1", "0.5.14-beta.2", "0.5.14-beta.2", false},
		{"0.5.14-beta.2", "0.5.14-beta.1", "", true},
		{"0.5.13", "0.5.12", "", true},
		{"0.5.13", "0.5.13", "", false},
	} {
		if CanInstall(Release{Version: test.next, SwitchFrom: test.from}, test.current) != test.allowed {
			t.Errorf("installation from %s to %s with switch from %q", test.current, test.next, test.from)
		}
	}
}

func TestChannelManifestSelection(t *testing.T) {
	beta := channelFixture(t, "0.5.14-beta.1", true)
	stable := channelFixture(t, "0.5.13", false)
	for _, test := range []struct {
		source        githubRelease
		current       string
		channel       Channel
		version, from string
	}{
		{beta, "0.5.13", Beta, beta.Tag, "0.5.13"},
		{stable, "0.5.14-beta.1", Stable, stable.Tag, "0.5.14-beta.1"},
		{beta, "0.5.14-beta.1", Beta, "", ""},
		{beta, "0.5.14-beta.2", Beta, "", ""},
		{beta, "0.5.13-beta.9", Beta, beta.Tag, ""},
	} {
		got, err := parseChannelRelease(test.source, test.current, test.channel)
		if err != nil || got.Version != test.version || got.SwitchFrom != test.from {
			t.Fatalf("channel selection: %+v %v", got, err)
		}
	}
	for _, source := range []githubRelease{stable, channelFixture(t, beta.Tag, false), channelFixture(t, "0.5.14-preview.1", true)} {
		if _, err := parseChannelRelease(source, "0.5.13", Beta); err == nil {
			t.Fatal("accepted a release outside the beta channel")
		}
	}
	for _, mutate := range []func(*githubRelease){
		func(r *githubRelease) { r.Draft = true },
		func(r *githubRelease) { r.Assets[0].Digest = "" },
		func(r *githubRelease) { r.Assets[0].URL = "https://example.com/beta.zip" },
		func(r *githubRelease) { r.Assets = append(r.Assets, r.Assets[0]) },
	} {
		source := channelFixture(t, beta.Tag, true)
		mutate(&source)
		if _, err := parseChannelRelease(source, "0.5.13", Beta); err == nil {
			t.Fatal("accepted invalid beta package")
		}
	}
}

func TestChannelFeedsAndPagination(t *testing.T) {
	stable := channelFixture(t, "0.5.13", false)
	first := make([]githubRelease, 100)
	for i := range first {
		first[i] = stable
	}
	first[0] = channelFixture(t, "0.5.14-beta.2", true)
	draft := channelFixture(t, "0.5.99-beta.1", true)
	draft.Draft = true
	second := []githubRelease{draft, channelFixture(t, "0.5.14-beta.10", true)}
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		if r.URL.Path == "/latest" {
			_ = json.NewEncoder(w).Encode(stable)
		} else if r.URL.Query().Get("page") == "1" {
			_ = json.NewEncoder(w).Encode(first)
		} else {
			_ = json.NewEncoder(w).Encode(second)
		}
	}))
	defer server.Close()
	got, err := checkChannel(context.Background(), server.Client(), "0.5.13", Beta, server.URL)
	if err != nil || got.Version != "0.5.14-beta.10" || len(requests) != 2 {
		t.Fatalf("beta feed: %+v %v %v", got, err, requests)
	}
	got, err = checkChannel(context.Background(), server.Client(), "0.5.14-beta.10", Stable, server.URL)
	if err != nil || got.Version != stable.Tag || got.SwitchFrom != "0.5.14-beta.10" || requests[2] != "/latest" {
		t.Fatalf("stable feed: %+v %v %v", got, err, requests)
	}
}
