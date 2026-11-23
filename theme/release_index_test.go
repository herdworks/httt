package theme

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
)

type indexRoundTripper func(*http.Request) (*http.Response, error)

func (f indexRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReleaseIndexURL(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{"https://example.test", "https://example.test/.hypertexting/themes.json"},
		{"https://example.test/", "https://example.test/.hypertexting/themes.json"},
		{"http://localhost:8080/?token=abc", "http://localhost:8080/.hypertexting/themes.json?token=abc"},
		{"https://example.test/.hypertexting/themes.json", "https://example.test/.hypertexting/themes.json"},
		{"https://example.test/custom/index?download=1", "https://example.test/custom/index?download=1"},
		{"https://example.test/custom/", "https://example.test/custom/"},
		{"https://example.test/my%20index.json?q=%2F", "https://example.test/my%20index.json?q=%2F"},
		{"https://example.test/%2F", "https://example.test/%2F"},
	} {
		t.Run(test.source, func(t *testing.T) {
			got, err := ReleaseIndexURL(test.source)
			if err != nil || got != test.want {
				t.Fatalf("ReleaseIndexURL = %q, %v; want %q", got, err, test.want)
			}
		})
	}
	for _, source := range []string{"", "example.test", "/index.json", "file:///index.json", "https://user:pass@example.test", "https://example.test/#fragment", "https://%"} {
		if _, err := ReleaseIndexURL(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}

func TestInstalledThemeResolvesIndexURLsAndArbitraryArchiveLocations(t *testing.T) {
	for _, source := range []string{"https://example.test", "https://example.test/", "https://example.test/custom/releases?channel=stable"} {
		t.Run(source, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "theme.json")
			writeThemeTestFileContent(t, path, `{"name":"Example","version":"0.1.0","config":{"url":"`+source+`"}}`)
			theme := &Theme{URI: &url.URL{Path: path}}
			if err := theme.Load(io.Discard); err != nil {
				t.Fatal(err)
			}
			requested := ""
			client := &http.Client{Transport: indexRoundTripper(func(r *http.Request) (*http.Response, error) {
				requested = r.URL.String()
				return &http.Response{StatusCode: 200, Header: make(http.Header), Request: r, Body: io.NopCloser(strings.NewReader(`{"name":"Example","latest":"2.0.0","versions":{"1.0.0":{"url":"../downloads/unpredictable.zip?file=1","sha256":"` + strings.Repeat("a", 64) + `"},"2.0.0":{"url":"https://cdn.example/different-name.zip","sha256":"` + strings.Repeat("b", 64) + `"}}}`))}, nil
			})}
			latest, err := theme.ResolveRelease(context.Background(), client, "")
			if err != nil {
				t.Fatal(err)
			}
			wantIndex, _ := ReleaseIndexURL(source)
			if requested != wantIndex {
				t.Fatalf("fetched %q, want %q", requested, wantIndex)
			}
			if latest.URL != "https://cdn.example/different-name.zip" || latest.SHA256 != strings.Repeat("b", 64) {
				t.Fatalf("latest = %#v", latest)
			}
			old, err := theme.ResolveRelease(context.Background(), client, "1.0.0")
			if err != nil || old.URL != "https://example.test/downloads/unpredictable.zip?file=1" {
				t.Fatalf("old = %#v, %v", old, err)
			}
			if _, err := theme.ResolveRelease(context.Background(), client, "missing"); err == nil {
				t.Fatal("accepted missing version")
			}
			theme.Name = "Other"
			if _, err := theme.ResolveRelease(context.Background(), client, ""); err == nil {
				t.Fatal("accepted wrong theme index")
			}
		})
	}
}

func TestFetchIndexResolvesArchivesAfterRedirect(t *testing.T) {
	client := &http.Client{Transport: indexRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == ReleaseIndexPath {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://cdn.example/catalog/index"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"name":"Example","latest":"1","versions":{"1":{"url":"archives/file.zip","sha256":"` + strings.Repeat("a", 64) + `"}}}`)), Request: r}, nil
	})}
	index, err := FetchReleaseIndex(context.Background(), client, "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	if index.Versions["1"].URL != "https://cdn.example/catalog/archives/file.zip" {
		t.Fatalf("index = %#v", index)
	}
}

func TestFetchIndexRejectsBadResponses(t *testing.T) {
	valid := `{"name":"Example","latest":"1","versions":{"1":{"url":"/different.zip","sha256":"` + strings.Repeat("a", 64) + `"}}}`
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"HTTP failure", valid, 404},
		{"bad JSON", "{", 200},
		{"trailing JSON", valid + ` {}`, 200},
		{"missing latest version", strings.Replace(valid, `"latest":"1"`, `"latest":"2"`, 1), 200},
		{"bad digest", strings.Replace(valid, strings.Repeat("a", 64), "sha256:no", 1), 200},
		{"empty archive URL", strings.Replace(valid, "/different.zip", "", 1), 200},
		{"non HTTP archive URL", strings.Replace(valid, "/different.zip", "file:///archive.zip", 1), 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: indexRoundTripper(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Status: http.StatusText(test.status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(test.body)), Request: r}, nil
			})}
			if _, err := FetchReleaseIndex(context.Background(), client, "https://example.test"); err == nil {
				t.Fatal("accepted bad index")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &http.Client{Transport: indexRoundTripper(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })}
	if _, err := FetchReleaseIndex(ctx, client, "https://example.test"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}
