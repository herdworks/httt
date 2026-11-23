package theme

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func releaseFixture(t *testing.T) (*Theme, string) {
	t.Helper()
	root := t.TempDir()
	theme := &Theme{URI: &url.URL{Path: filepath.Join(root, "themes", "example", "theme.json")}}
	writeThemeTestFileContent(t, theme.URI.Path, `{"name":"Example Theme","version":"1.0.0","config":{"layouts_dir":"templates"}}`)
	writeThemeTestFileContent(t, filepath.Join(theme.RootDir(), "templates", "default.html"), "<main>example</main>")
	return theme, filepath.Join(root, "static")
}

func readReleaseFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readReleaseIndex(t *testing.T, static string) ReleaseIndex {
	t.Helper()
	var index ReleaseIndex
	if err := json.Unmarshal(readReleaseFile(t, filepath.Join(static, ".hypertexting", "themes.json")), &index); err != nil {
		t.Fatal(err)
	}
	if err := index.Validate(); err != nil {
		t.Fatal(err)
	}
	return index
}

func assertReleaseUnchanged(t *testing.T, static, archive string, indexBytes, archiveBytes []byte) {
	t.Helper()
	if got := readReleaseFile(t, filepath.Join(static, ".hypertexting", "themes.json")); !reflect.DeepEqual(got, indexBytes) {
		t.Fatal("failed release changed index")
	}
	if got := readReleaseFile(t, archive); !reflect.DeepEqual(got, archiveBytes) {
		t.Fatal("failed release changed existing archive")
	}
}

func TestReleaseLifecyclePreservesHistoryAndMetadata(t *testing.T) {
	theme, static := releaseFixture(t)
	first, err := theme.Release(static, "")
	if err != nil {
		t.Fatal(err)
	}
	if first.URL != "/.hypertexting/themes/example-theme-v1.0.0.zip" {
		t.Fatalf("release = %#v", first)
	}
	archive := filepath.Join(static, filepath.FromSlash(strings.TrimPrefix(first.URL, "/")))
	archiveBytes := readReleaseFile(t, archive)
	hash := sha256.Sum256(archiveBytes)
	if first.SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatal("digest does not match exact ZIP bytes")
	}
	index := readReleaseIndex(t, static)
	if index.Name != "Example Theme" || index.Latest != "1.0.0" || index.Versions["1.0.0"] != first {
		t.Fatalf("index = %#v", index)
	}
	// The release must retain native archive layout and configured directories.
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]bool{}
	for _, entry := range reader.File {
		entries[entry.Name] = true
	}
	reader.Close()
	if !entries["theme.json"] || !entries["templates/default.html"] {
		t.Fatalf("entries = %#v", entries)
	}
	native := filepath.Join(t.TempDir(), "native.zip")
	source := &Theme{URI: theme.URI}
	if err := source.Load(io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := source.Archive(native, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(archiveBytes, readReleaseFile(t, native)) {
		t.Fatal("release differs from native Theme.Archive bytes")
	}
	// Identical-byte retries succeed, without rewriting the archive.
	info, _ := os.Stat(archive)
	if retry, err := theme.Release(static, ""); err != nil || retry != first {
		t.Fatalf("retry = %#v, %v", retry, err)
	}
	after, _ := os.Stat(archive)
	if !os.SameFile(info, after) {
		t.Fatal("retry replaced archive inode")
	}
	indexPath := filepath.Join(static, ".hypertexting", "themes.json")
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(readReleaseFile(t, indexPath), &doc); err != nil {
		t.Fatal(err)
	}
	doc["description"] = json.RawMessage(`"keep me"`)
	var versions map[string]json.RawMessage
	if err := json.Unmarshal(doc["versions"], &versions); err != nil {
		t.Fatal(err)
	}
	versions["0.9.0"] = json.RawMessage(`{"url":"https://cdn.example/old.zip","sha256":"` + strings.Repeat("a", 64) + `","notes":{"keep":true}}`)
	doc["versions"], _ = json.Marshal(versions)
	data, _ := json.Marshal(doc)
	writeThemeTestFileContent(t, indexPath, string(data))
	writeThemeTestFileContent(t, theme.URI.Path, `{"name":"Example Theme","version":"2.0.0","config":{"layouts_dir":"templates"}}`)
	second, err := theme.Release(static, "downloads/custom-name.zip")
	if err != nil {
		t.Fatal(err)
	}
	index = readReleaseIndex(t, static)
	if index.Latest != "2.0.0" || len(index.Versions) != 3 || index.Versions["1.0.0"] != first || second.URL != "/downloads/custom-name.zip" {
		t.Fatalf("index = %#v; release = %#v", index, second)
	}
	if !reflect.DeepEqual(archiveBytes, readReleaseFile(t, archive)) {
		t.Fatal("new version changed old archive")
	}
	if err := json.Unmarshal(readReleaseFile(t, indexPath), &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc["description"]) != `"keep me"` || !strings.Contains(string(doc["versions"]), `"keep": true`) {
		t.Fatalf("metadata lost: %s", readReleaseFile(t, indexPath))
	}
}

func TestReleaseRejectsSameVersionConflictsWithoutChangingIndex(t *testing.T) {
	for _, conflict := range []string{"source bytes", "archive bytes", "index digest", "different theme", "malformed index", "invalid index", "locked"} {
		t.Run(conflict, func(t *testing.T) {
			theme, static := releaseFixture(t)
			first, err := theme.Release(static, "")
			if err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(static, filepath.FromSlash(strings.TrimPrefix(first.URL, "/")))
			indexPath := filepath.Join(static, ".hypertexting", "themes.json")
			switch conflict {
			case "source bytes":
				writeThemeTestFileContent(t, filepath.Join(theme.RootDir(), "templates", "default.html"), "changed")
			case "archive bytes":
				writeThemeTestFileContent(t, archive, "corrupted")
			case "index digest":
				data := strings.ReplaceAll(string(readReleaseFile(t, indexPath)), first.SHA256, strings.Repeat("b", 64))
				writeThemeTestFileContent(t, indexPath, data)
			case "different theme":
				writeThemeTestFileContent(t, theme.URI.Path, `{"name":"Another Theme","version":"1.0.0"}`)
			case "malformed index":
				writeThemeTestFileContent(t, indexPath, `{`)
			case "invalid index":
				writeThemeTestFileContent(t, indexPath, `{"name":"Example Theme","latest":"missing","versions":{}}`)
			case "locked":
				writeThemeTestFileContent(t, filepath.Join(static, ".hypertexting", ".theme-release.lock"), "")
			}
			indexBytes, archiveBytes := readReleaseFile(t, indexPath), readReleaseFile(t, archive)
			if _, err := theme.Release(static, ""); err == nil {
				t.Fatal("expected conflict")
			}
			assertReleaseUnchanged(t, static, archive, indexBytes, archiveBytes)
		})
	}
}

func TestReleaseFailureBeforeArchiveLeavesNoIndex(t *testing.T) {
	for _, failure := range []string{"missing metadata", "bad version", "unsafe name", "unsafe output", "blocked archive", "bad resource"} {
		t.Run(failure, func(t *testing.T) {
			theme, static := releaseFixture(t)
			output := ""
			switch failure {
			case "missing metadata":
				writeThemeTestFileContent(t, theme.URI.Path, `{}`)
			case "bad version":
				writeThemeTestFileContent(t, theme.URI.Path, `{"name":"example","version":"../outside"}`)
			case "unsafe name":
				writeThemeTestFileContent(t, theme.URI.Path, `{"name":"../example","version":"1.0.0"}`)
			case "unsafe output":
				output = "../outside.zip"
			case "blocked archive":
				writeThemeTestFileContent(t, filepath.Join(static, ".hypertexting", "themes"), "not a directory")
			case "bad resource":
				writeThemeTestFileContent(t, theme.URI.Path, `{"name":"Example Theme","version":"1.0.0","config":{"layouts_dir":"bad-templates"}}`)
				writeThemeTestFileContent(t, filepath.Join(theme.RootDir(), "bad-templates"), "not a directory")
			}
			if _, err := theme.Release(static, output); err == nil {
				t.Fatal("expected release failure")
			}
			if _, err := os.Stat(filepath.Join(static, ".hypertexting", "themes.json")); !os.IsNotExist(err) {
				t.Fatalf("index exists or unexpected error: %v", err)
			}
			stages, _ := filepath.Glob(filepath.Join(static, ".hypertexting", ".theme-release-*"))
			if len(stages) != 0 {
				t.Fatalf("left staged files: %v", stages)
			}
		})
	}
}

func TestReleaseRecoversUnindexedIdenticalArchive(t *testing.T) {
	theme, static := releaseFixture(t)
	first, err := theme.Release(static, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(static, ".hypertexting", "themes.json")); err != nil {
		t.Fatal(err)
	}
	if recovered, err := theme.Release(static, ""); err != nil || recovered != first {
		t.Fatalf("recovery = %#v, %v", recovered, err)
	}
}

func TestReleaseRelativeStaticOverlapExcludesAllPublishedArchives(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	theme := &Theme{URI: &url.URL{Path: "theme.json"}}
	writeThemeTestFileContent(t, "theme.json", `{"name":"Overlap","version":"1.0.0"}`)
	writeThemeTestFileContent(t, "static/site.css", "body{}")
	// Old custom paths must remain excluded when a newer version is packaged.
	if _, err := theme.Release("static", "downloads/old.zip"); err != nil {
		t.Fatal(err)
	}
	if _, err := theme.Release("static", "downloads/old.zip"); err != nil {
		t.Fatalf("custom retry: %v", err)
	}
	writeThemeTestFileContent(t, "theme.json", `{"name":"Overlap","version":"2.0.0"}`)
	first, err := theme.Release("static", "")
	if err != nil {
		t.Fatal(err)
	}
	if retry, err := theme.Release("static", ""); err != nil || retry != first {
		t.Fatalf("overlapping retry = %#v, %v", retry, err)
	}
	reader, err := zip.OpenReader("static" + first.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	found := false
	for _, entry := range reader.File {
		if strings.Contains(entry.Name, ".hypertexting") || strings.HasSuffix(entry.Name, ".zip") {
			t.Fatalf("publication output packaged: %s", entry.Name)
		}
		if entry.Name == "static/site.css" {
			found = true
		}
	}
	if !found {
		t.Fatal("theme static asset omitted")
	}
}

func TestReleaseRejectsRelocationAndPreservesIndexedVersion(t *testing.T) {
	theme, static := releaseFixture(t)
	first, err := theme.Release(static, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(static, ".hypertexting", "themes.json")
	archive := filepath.Join(static, filepath.FromSlash(strings.TrimPrefix(first.URL, "/")))
	data := strings.Replace(string(readReleaseFile(t, path)), `"sha256":`, `"notes": "retain", "sha256":`, 1)
	writeThemeTestFileContent(t, path, data)
	indexBytes, archiveBytes := readReleaseFile(t, path), readReleaseFile(t, archive)
	if _, err := theme.Release(static, "download/renamed.zip"); err == nil || !strings.Contains(err.Error(), "different archive path") {
		t.Fatalf("relocation error = %v", err)
	}
	assertReleaseUnchanged(t, static, archive, indexBytes, archiveBytes)
	if _, err := os.Stat(filepath.Join(static, "download")); !os.IsNotExist(err) {
		t.Fatalf("relocation created an output directory: %v", err)
	}
	if retry, err := theme.Release(static, ""); err != nil || retry != first {
		t.Fatalf("same-path retry = %#v, %v", retry, err)
	}
	if !strings.Contains(string(readReleaseFile(t, path)), `"notes": "retain"`) {
		t.Fatal("retry lost record metadata")
	}
}

func TestVerifyReleaseArchiveChecksMetadataAndAllEntryChecksums(t *testing.T) {
	for _, failure := range []string{"wrong metadata", "missing metadata", "corrupt resource"} {
		t.Run(failure, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "staged.zip")
			var buffer strings.Builder
			writer := zip.NewWriter(&buffer)
			if failure != "missing metadata" {
				entry, err := writer.Create("theme.json")
				if err != nil {
					t.Fatal(err)
				}
				metadata := `{"name":"Example","version":"1"}`
				if failure == "wrong metadata" {
					metadata = `{"name":"Other","version":"1"}`
				}
				if _, err := io.WriteString(entry, metadata); err != nil {
					t.Fatal(err)
				}
			}
			entry, err := writer.CreateHeader(&zip.FileHeader{Name: "static/data.txt", Method: zip.Store})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(entry, "resource bytes"); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			data := buffer.String()
			if failure == "corrupt resource" {
				data = strings.Replace(data, "resource bytes", "corrupt! bytes", 1)
			}
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := verifyReleaseArchive(path, "Example", "1"); err == nil {
				t.Fatal("accepted invalid staged archive")
			}
		})
	}
}

func TestReleaseExcludesRelativePriorCustomURL(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	theme := &Theme{URI: &url.URL{Path: "theme.json"}}
	writeThemeTestFileContent(t, "theme.json", `{"name":"Overlap","version":"1"}`)
	first, err := theme.Release("static", "downloads/prior.zip")
	if err != nil {
		t.Fatal(err)
	}
	indexPath := "static/.hypertexting/themes.json"
	data := strings.Replace(string(readReleaseFile(t, indexPath)), first.URL, "../downloads/prior.zip", 1)
	writeThemeTestFileContent(t, indexPath, data)
	// A retry must omit the existing custom archive even when its index URL
	// is relative to the index directory rather than relative to the origin.
	first.URL = "../downloads/prior.zip"
	if retry, err := theme.Release("static", "downloads/prior.zip"); err != nil || retry != first {
		t.Fatalf("retry = %#v, %v", retry, err)
	}
}

func TestReleasePriorDirectoryURLDoesNotExcludeRealAssets(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	theme := &Theme{URI: &url.URL{Path: "theme.json"}}
	writeThemeTestFileContent(t, "theme.json", `{"name":"Overlap","version":"1"}`)
	writeThemeTestFileContent(t, "static/site.css", "body{}")
	first, err := theme.Release("static", "")
	if err != nil {
		t.Fatal(err)
	}
	indexPath := "static/.hypertexting/themes.json"
	data := strings.Replace(string(readReleaseFile(t, indexPath)), first.URL, "/", 1)
	writeThemeTestFileContent(t, indexPath, data)
	writeThemeTestFileContent(t, "theme.json", `{"name":"Overlap","version":"2"}`)
	second, err := theme.Release("static", "")
	if err != nil {
		t.Fatalf("directory URL changed packaging of real assets: %v", err)
	}
	reader, err := zip.OpenReader("static" + second.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	found := false
	for _, entry := range reader.File {
		if entry.Name == "static/site.css" {
			found = true
		}
	}
	if !found {
		t.Fatal("directory URL excluded genuine static assets")
	}
}

func TestReleaseRejectsMetadataOutputCollisions(t *testing.T) {
	for _, output := range []string{".hypertexting", ".hypertexting/themes.json", ".hypertexting/.theme-release.lock"} {
		t.Run(output, func(t *testing.T) {
			theme, static := releaseFixture(t)
			if _, err := theme.Release(static, output); err == nil {
				t.Fatal("accepted metadata output collision")
			}
			if _, err := os.Stat(filepath.Join(static, ".hypertexting")); !os.IsNotExist(err) {
				t.Fatalf("created metadata on rejected output: %v", err)
			}
		})
	}
}
