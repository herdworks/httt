package theme

import (
	"archive/zip"
	"bytes"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverlappingReleasePreservesOtherHypertextingAssets(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	theme := &Theme{URI: &url.URL{Path: "theme.json"}}
	assets := map[string]string{
		"static/.hypertexting/site-config.json":  `{"site":"real asset"}`,
		"static/.hypertexting/assets/site.css":   "body{}",
		"static/.hypertexting/themes/readme.txt": "theme catalog instructions",
		"static/.hypertexting/other.zip":         "a legitimate non-release asset",
	}
	for path, content := range assets {
		writeThemeTestFileContent(t, path, content)
	}
	for _, version := range []string{"1", "1", "2", "2"} {
		// Preserve source ZIP metadata on retries.
		metadata := `{"name":"Overlap","version":"` + version + `"}`
		if data, _ := os.ReadFile("theme.json"); string(data) != metadata {
			writeThemeTestFileContent(t, "theme.json", metadata)
		}
		archivePath := ".hypertexting/themes/overlap-v" + version + ".zip"
		if version == "1" {
			archivePath = "custom/deep/old.zip"
		}
		release, err := theme.Release("static", archivePath)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := zip.OpenReader("static" + release.URL)
		if err != nil {
			t.Fatal(err)
		}
		found := map[string]bool{}
		for _, entry := range reader.File {
			if want, ok := assets[entry.Name]; ok {
				file, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(file)
				file.Close()
				if err != nil || string(got) != want {
					t.Fatalf("asset %s = %q, %v", entry.Name, got, err)
				}
				found[entry.Name] = true
				continue
			}
			if entry.Name == "static/.hypertexting/themes.json" || entry.Name == "static/.hypertexting/.theme-release.lock" || strings.Contains(entry.Name, ".theme-release-") || strings.HasSuffix(entry.Name, ".zip") {
				t.Fatalf("release artifact packaged: %s", entry.Name)
			}
		}
		reader.Close()
		if len(found) != len(assets) {
			t.Fatalf("missing genuine assets: found %#v", found)
		}
	}
}

func TestReleaseRejectsSymlinkOutputsWithoutChangingTargets(t *testing.T) {
	for _, test := range []struct {
		name, link, archive string
		file                bool
	}{
		{"static root", ".", "", false},
		{"static root trailing slash", ".", "", false},
		{"manifest parent", ".hypertexting", "", false},
		{"manifest", ".hypertexting/themes.json", "", true},
		{"lock", ".hypertexting/.theme-release.lock", "", true},
		{"default archive ancestor", ".hypertexting/themes", "", false},
		{"default archive destination", ".hypertexting/themes/example-theme-v1.0.0.zip", "", true},
		{"custom archive ancestor", "downloads", "downloads/deep/version.zip", false},
		{"nested custom ancestor", "downloads/deep", "downloads/deep/version.zip", false},
		{"custom archive destination", "downloads/deep/version.zip", "downloads/deep/version.zip", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			theme, static := releaseFixture(t)
			fixture := t.TempDir()
			target := filepath.Join(fixture, "target")
			if test.file {
				writeThemeTestFileContent(t, target, "fixture bytes")
			} else {
				if err := os.MkdirAll(target, 0o755); err != nil {
					t.Fatal(err)
				}
				writeThemeTestFileContent(t, filepath.Join(target, "sentinel"), "fixture bytes")
			}
			link := filepath.Join(static, filepath.FromSlash(test.link))
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			before := snapshotReleaseFixture(t, fixture)
			staticInput := static
			if test.name == "static root trailing slash" {
				staticInput += string(filepath.Separator)
			}
			if _, err := theme.Release(staticInput, test.archive); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("symlink error = %v", err)
			}
			assertFixtureSnapshot(t, fixture, before)
			info, err := os.Lstat(link)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("release changed symlink: %v", err)
			}
		})
	}
}

func TestReleaseRejectsIndexedArchiveSymlinkEvenWithMatchingBytes(t *testing.T) {
	for _, destination := range []string{"current", "prior"} {
		t.Run(destination, func(t *testing.T) {
			theme, static := releaseFixture(t)
			first, err := theme.Release(static, "downloads/first.zip")
			if err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(static, "downloads", "first.zip")
			archiveBytes := readReleaseFile(t, archive)
			fixture := t.TempDir()
			target := filepath.Join(fixture, "identical.zip")
			if err := os.WriteFile(target, archiveBytes, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(archive); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, archive); err != nil {
				t.Fatal(err)
			}
			outputs, err := openReleaseRoot(static)
			if err != nil {
				t.Fatal(err)
			}
			_, digestErr := releaseArchiveDigest(outputs, "downloads/first.zip")
			outputs.Close()
			if digestErr == nil || !strings.Contains(digestErr.Error(), "symlink") {
				t.Fatalf("digest symlink error = %v", digestErr)
			}
			output := "downloads/first.zip"
			if destination == "prior" {
				writeThemeTestFileContent(t, theme.URI.Path, `{"name":"Example Theme","version":"2.0.0","config":{"layouts_dir":"templates"}}`)
				output = "downloads/second.zip"
			}
			indexPath := filepath.Join(static, ".hypertexting", "themes.json")
			indexBytes := readReleaseFile(t, indexPath)
			if _, err := theme.Release(static, output); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("indexed symlink error = %v (digest %s)", err, first.SHA256)
			}
			if !bytes.Equal(indexBytes, readReleaseFile(t, indexPath)) || !bytes.Equal(archiveBytes, readReleaseFile(t, target)) {
				t.Fatal("rejection changed index or fixture archive")
			}
		})
	}
}

// All symlink targets and snapshots stay in temporary fixtures. Verify that a
// rejected release leaves their full file tree and bytes untouched.
func snapshotReleaseFixture(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[relative] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[relative] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertFixtureSnapshot(t *testing.T, root string, before map[string]string) {
	t.Helper()
	after := snapshotReleaseFixture(t, root)
	if len(before) != len(after) {
		t.Fatalf("fixture tree changed: before %v, after %v", before, after)
	}
	for path, data := range before {
		if value, exists := after[path]; !exists || value != data {
			t.Fatalf("fixture %s changed", path)
		}
	}
}

func TestStableReleaseArchivePathComparison(t *testing.T) {
	for _, test := range []struct {
		indexed string
		same    bool
	}{
		{"/.hypertexting/themes/a.zip", true},
		{"themes/a.zip", true},
		{"./themes/a.zip", true},
		{"../.hypertexting/themes/a.zip", true},
		{"/.hypertexting/elsewhere/a.zip", false},
		{"themes/a.zip?download=1", false},
		{"https://example.test/.hypertexting/themes/a.zip", false},
		{"//example.test/.hypertexting/themes/a.zip", false},
		{"themes%2Fa.zip", false},
	} {
		if got := sameReleaseArchivePath(test.indexed, "/.hypertexting/themes/a.zip"); got != test.same {
			t.Errorf("sameReleaseArchivePath(%q) = %v", test.indexed, got)
		}
	}
}

func TestReleaseRejectsInternalRelativeSymlinkAncestor(t *testing.T) {
	theme, static := releaseFixture(t)
	writeThemeTestFileContent(t, filepath.Join(static, "real-downloads", "sentinel"), "fixture bytes")
	if err := os.Symlink("real-downloads", filepath.Join(static, "downloads")); err != nil {
		t.Fatal(err)
	}
	before := snapshotReleaseFixture(t, filepath.Join(static, "real-downloads"))
	if _, err := theme.Release(static, "downloads/nested/archive.zip"); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("internal symlink error = %v", err)
	}
	assertFixtureSnapshot(t, filepath.Join(static, "real-downloads"), before)
}
