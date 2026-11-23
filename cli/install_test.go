package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herdworks/clic"
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

func TestInstallLatestLocalAndHTTP(t *testing.T) {
	for _, remote := range []bool{false, true} {
		for _, existing := range []bool{false, true} {
			t.Run(fmt.Sprintf("remote=%t/existing=%t", remote, existing), func(t *testing.T) {
				root := t.TempDir()
				config := writeSiteConfig(t, root, "theme.json", "themes")
				latest := filepath.Join(root, "themes", "example", "latest")
				if existing {
					writeCLIFile(t, filepath.Join(latest, "old.txt"), "old")
					outside := filepath.Join(root, "outside")
					writeCLIFile(t, filepath.Join(outside, "keep.txt"), "keep")
					if err := os.Symlink(outside, filepath.Join(latest, "link")); err != nil {
						t.Fatal(err)
					}
				}
				// Neither sibling versions nor other themes may be deleted.
				sibling := filepath.Join(root, "themes", "example", "0.9.0", "keep.txt")
				other := filepath.Join(root, "themes", "other", "latest", "keep.txt")
				writeCLIFile(t, sibling, "keep")
				writeCLIFile(t, other, "keep")
				metadata := `{"name":"example","version":"1.2.3"}`
				archive := writeThemePackage(t, filepath.Join(root, "theme.zip"), metadata, map[string]string{
					"layouts/default.html": "new layout",
				})
				if remote {
					archive = serveThemePackage(t, archive)
				}
				output, err := runCLI(t, "install", "--config", config, "--latest", archive)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(output, `version "1.2.3" installed successfully`) {
					t.Fatalf("unexpected output: %s", output)
				}
				assertInstallFile(t, filepath.Join(latest, "theme.json"), metadata)
				assertInstallFile(t, filepath.Join(latest, "layouts", "default.html"), "new layout")
				assertInstallMissing(t, filepath.Join(latest, "old.txt"))
				assertInstallMissing(t, filepath.Join(root, "themes", "example", "1.2.3"))
				assertInstallFile(t, sibling, "keep")
				assertInstallFile(t, other, "keep")
				if existing {
					assertInstallFile(t, filepath.Join(root, "outside", "keep.txt"), "keep")
				}
			})
		}
	}
}

func TestInstallLatestFailedUnpackPreservesLatest(t *testing.T) {
	for _, remote := range []bool{false, true} {
		t.Run(fmt.Sprintf("remote=%t", remote), func(t *testing.T) {
			root := t.TempDir()
			config := writeSiteConfig(t, root, "theme.json", "themes")
			latest := filepath.Join(root, "themes", "example", "latest")
			writeCLIFile(t, filepath.Join(latest, "theme.json"), "old metadata")
			writeCLIFile(t, filepath.Join(latest, "layouts", "default.html"), "old layout")
			archive := writeThemePackage(t, filepath.Join(root, "broken.zip"), `{"name":"example","version":"1.2.3"}`, map[string]string{
				"../escape.txt": "must not escape",
			})
			if remote {
				archive = serveThemePackage(t, archive)
			}
			output, err := runCLI(t, "install", "--config", config, "--latest", archive)
			if err == nil || !strings.Contains(err.Error(), "extract theme package") {
				t.Fatalf("error = %v; output = %s", err, output)
			}
			assertInstallFile(t, filepath.Join(latest, "theme.json"), "old metadata")
			assertInstallFile(t, filepath.Join(latest, "layouts", "default.html"), "old layout")
			assertInstallMissing(t, filepath.Join(root, "themes", "example", "1.2.3"))
			assertInstallMissing(t, filepath.Join(root, "themes", "example", "escape.txt"))
			// A corrected package of the same version must work without manual cleanup.
			metadata := `{"name":"example","version":"1.2.3"}`
			archive = writeThemePackage(t, filepath.Join(root, "corrected.zip"), metadata, map[string]string{
				"layouts/default.html": "corrected layout",
			})
			if remote {
				archive = serveThemePackage(t, archive)
			}
			if _, err := runCLI(t, "install", "--config", config, "--latest", archive); err != nil {
				t.Fatalf("same-version retry failed: %v", err)
			}
			assertInstallFile(t, filepath.Join(latest, "theme.json"), metadata)
			assertInstallFile(t, filepath.Join(latest, "layouts", "default.html"), "corrected layout")
			assertInstallMissing(t, filepath.Join(root, "themes", "example", "1.2.3"))
		})
	}
}

func TestInstallLatestRejectsConflicts(t *testing.T) {
	for _, conflict := range []string{"latest version", "case alias", "existing version", "version file", "version symlink", "dangling version symlink", "latest file", "latest symlink", "dangling latest symlink", "theme symlink"} {
		t.Run(conflict, func(t *testing.T) {
			root := t.TempDir()
			config := writeSiteConfig(t, root, "theme.json", "themes")
			target := filepath.Join(root, "themes", "example")
			outside := filepath.Join(root, "outside")
			writeCLIFile(t, filepath.Join(outside, "keep.txt"), "keep")
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			version := "1.2.3"
			link := func(source, destination string) {
				t.Helper()
				if err := os.Symlink(source, destination); err != nil {
					t.Fatal(err)
				}
			}
			switch conflict {
			case "latest version":
				version = "latest"
			case "case alias":
				version = "LATEST"
			case "existing version":
				writeCLIFile(t, filepath.Join(target, version, "keep.txt"), "keep")
			case "version file":
				writeCLIFile(t, filepath.Join(target, version), "keep")
			case "version symlink":
				link(outside, filepath.Join(target, version))
			case "dangling version symlink":
				link(filepath.Join(root, "missing"), filepath.Join(target, version))
			case "latest file":
				writeCLIFile(t, filepath.Join(target, "latest"), "keep")
			case "latest symlink":
				link(outside, filepath.Join(target, "latest"))
			case "dangling latest symlink":
				link(filepath.Join(root, "missing"), filepath.Join(target, "latest"))
			case "theme symlink":
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				link(outside, target)
			}
			hasLatestDirectory := conflict != "latest file" && conflict != "latest symlink" && conflict != "dangling latest symlink" && conflict != "theme symlink"
			if hasLatestDirectory {
				writeCLIFile(t, filepath.Join(target, "latest", "keep.txt"), "keep")
			}
			archive := writeThemePackage(t, filepath.Join(root, "theme.zip"), fmt.Sprintf(`{"name":"example","version":%q}`, version), nil)
			if _, err := runCLI(t, "install", "--config", config, "--latest", archive); err == nil {
				t.Fatal("accepted conflicting install target")
			}
			assertInstallFile(t, filepath.Join(outside, "keep.txt"), "keep")
			assertInstallMissing(t, filepath.Join(outside, "theme.json"))
			if conflict == "latest file" {
				assertInstallFile(t, filepath.Join(target, "latest"), "keep")
			}
			if hasLatestDirectory {
				assertInstallFile(t, filepath.Join(target, "latest", "keep.txt"), "keep")
			}
			if conflict == "version file" {
				assertInstallFile(t, filepath.Join(target, version), "keep")
			}
			if strings.Contains(conflict, "symlink") {
				path := filepath.Join(target, "latest")
				if strings.Contains(conflict, "version") {
					path = filepath.Join(target, version)
				} else if conflict == "theme symlink" {
					path = target
				}
				if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("symlink changed: %v, %v", info, err)
				}
			}
		})
	}
}

func TestInstallWithoutLatestPreservesLatestAndRejectsDuplicate(t *testing.T) {
	root := t.TempDir()
	config := writeSiteConfig(t, root, "theme.json", "themes")
	latest := filepath.Join(root, "themes", "example", "latest", "keep.txt")
	writeCLIFile(t, latest, "keep")
	metadata := `{"name":"example","version":"1.2.3"}`
	archive := writeThemePackage(t, filepath.Join(root, "theme.zip"), metadata, nil)
	if _, err := runCLI(t, "install", "--config", config, archive); err != nil {
		t.Fatal(err)
	}
	assertInstallFile(t, filepath.Join(root, "themes", "example", "1.2.3", "theme.json"), metadata)
	assertInstallFile(t, latest, "keep")
	if _, err := runCLI(t, "install", "--config", config, archive); err == nil {
		t.Fatal("unflagged duplicate install succeeded")
	}
	assertInstallFile(t, latest, "keep")
}

func TestPackageAndDownstreamThemeInstallLatest(t *testing.T) {
	root := t.TempDir()
	config := writeSiteConfig(t, root, "theme.json", "themes")
	metadata := `{"name":"example","version":"1.2.3"}`
	writeCLIFile(t, filepath.Join(root, "source", "theme.json"), metadata)
	writeCLIFile(t, filepath.Join(root, "source", "layouts", "default.html"), "layout")
	archive := filepath.Join(root, "theme.zip")
	if _, err := runCLI(t, "package", "--config", config, "--theme", "source/theme.json", "--file", archive); err != nil {
		t.Fatal(err)
	}
	// Match hyperctl ThemeCommand: embed Command under the theme key.
	downstream := clic.CLI{
		Command: clic.Command{
			Spec: clic.CommandSpec{
				Key: "hyperctl",
			},
			Commands: clic.MustCommandsFrom(func() clic.Command {
				theme := Command()
				theme.Spec.Key = "theme"
				theme.Meta.EnvPrefix = ""
				return theme
			}),
		},
	}
	result := downstream.Run(clic.Execution{
		Context: context.Background(),
		Stdout:  &bytes.Buffer{},
		Stderr:  &bytes.Buffer{},
	}, []string{"theme", "install", "--latest", "--config", config, archive})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	assertInstallFile(t, filepath.Join(root, "themes", "example", "latest", "layouts", "default.html"), "layout")
	assertInstallFile(t, filepath.Join(root, "themes", "example", "latest", "theme.json"), metadata)
}

func serveThemePackage(t *testing.T, archive string) string {
	t.Helper()
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	previous := http.DefaultClient.Transport
	http.DefaultClient.Transport = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(data)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})
	t.Cleanup(func() { http.DefaultClient.Transport = previous })
	return "https://example.test/theme.zip"
}

func assertInstallFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != want {
		t.Fatalf("%s = %q, %v; want %q", path, data, err, want)
	}
}

func assertInstallMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("%s should not exist: %v", path, err)
	}
}

func TestCleanupFailedVersionPreservesReplacedTargetAndOriginalError(t *testing.T) {
	root := t.TempDir()
	version := filepath.Join(root, "1.2.3")
	writeCLIFile(t, filepath.Join(version, "partial.txt"), "partial")
	created, err := os.Lstat(version)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(version, filepath.Join(root, "moved")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	writeCLIFile(t, filepath.Join(outside, "keep.txt"), "keep")
	if err := os.Symlink(outside, version); err != nil {
		t.Fatal(err)
	}
	target, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	original := errors.New("unpack failed")
	err = cleanupFailedVersion(target, "1.2.3", created, original)
	if !errors.Is(err, original) || !strings.Contains(err.Error(), "clean up partial theme version") {
		t.Fatalf("cleanup error = %v; want original error and cleanup failure", err)
	}
	assertInstallFile(t, filepath.Join(outside, "keep.txt"), "keep")
	assertInstallFile(t, filepath.Join(root, "moved", "partial.txt"), "partial")
	if _, err := os.Readlink(version); err != nil {
		t.Fatalf("replacement symlink was removed: %v", err)
	}
}
