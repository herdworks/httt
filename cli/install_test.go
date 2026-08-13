package cli

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadThemeFromZipRejectsUnsafeDestinationComponents(t *testing.T) {
	tests := []struct {
		name  string
		theme string
		want  string
	}{
		{
			name:  "parent name",
			theme: `{"name":"../outside","version":"1.0.0"}`,
			want:  "package theme name",
		},
		{
			name:  "nested name",
			theme: `{"name":"nested/theme","version":"1.0.0"}`,
			want:  "package theme name",
		},
		{
			name:  "parent version",
			theme: `{"name":"theme","version":"../outside"}`,
			want:  "package theme version",
		},
		{
			name:  "nested version",
			theme: `{"name":"theme","version":"releases/1.0.0"}`,
			want:  "package theme version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := openThemePackage(t, test.theme)
			defer reader.Close()

			_, err := readThemeFromZip(reader)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("readThemeFromZip() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestReadThemeFromZipAcceptsSafeDestinationComponents(t *testing.T) {
	reader := openThemePackage(t, `{"name":"theme","version":"1.0.0"}`)
	defer reader.Close()

	theme, err := readThemeFromZip(reader)
	if err != nil {
		t.Fatalf("readThemeFromZip() error = %v", err)
	}
	if theme.Name != "theme" || theme.Version != "1.0.0" {
		t.Fatalf("readThemeFromZip() = %#v", theme)
	}
}

func TestInstallCommandInstallsLocalAndHTTPArchives(t *testing.T) {
	root := t.TempDir()
	config := writeSiteConfig(t, root, "theme.json", "themes")
	localArchive := writeThemePackage(t, filepath.Join(root, "local.zip"), `{"name":"local","version":"1.0.0"}`, map[string]string{
		"layouts/default.html": "<main>local</main>",
	})
	if _, err := runCLI(t, "install", "--config", config, localArchive); err != nil {
		t.Fatalf("local install error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "themes", "local", "1.0.0", "layouts", "default.html")); err != nil {
		t.Fatal(err)
	}

	httpArchive := writeThemePackage(t, filepath.Join(root, "http.zip"), `{"name":"remote","version":"2.0.0"}`, nil)
	archiveBytes, err := os.ReadFile(httpArchive)
	if err != nil {
		t.Fatal(err)
	}
	previousTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(archiveBytes)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})
	t.Cleanup(func() { http.DefaultClient.Transport = previousTransport })
	if _, err := runCLI(t, "install", "--config", config, "https://example.test/theme.zip"); err != nil {
		t.Fatalf("HTTP install error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "themes", "remote", "2.0.0", "theme.json")); err != nil {
		t.Fatal(err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (roundTripper roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTripper(request)
}

func TestExtractZipRejectsEscapingEntries(t *testing.T) {
	root := t.TempDir()
	archive := writeThemePackage(t, filepath.Join(root, "escape.zip"), `{"name":"theme","version":"1.0.0"}`, map[string]string{
		"../outside.txt": "escape",
	})
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	destination := filepath.Join(root, "themes", "theme", "1.0.0")
	if err = extractZip(reader, destination); err == nil {
		t.Fatal("extractZip() accepted an escaping archive entry")
	}
	if _, err = os.Stat(filepath.Join(root, "themes", "theme", "outside.txt")); !os.IsNotExist(err) {
		t.Fatalf("escaping archive entry was written: %v", err)
	}
}

func openThemePackage(t *testing.T, themeJSON string) *zip.ReadCloser {
	t.Helper()
	path := writeThemePackage(t, filepath.Join(t.TempDir(), "theme.zip"), themeJSON, nil)
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func writeThemePackage(t *testing.T, path, themeJSON string, entries map[string]string) string {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("theme.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = entry.Write([]byte(themeJSON)); err != nil {
		t.Fatal(err)
	}
	for name, content := range entries {
		entry, err = writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = fmt.Fprint(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
