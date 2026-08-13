package cli

import (
	"archive/zip"
	"path/filepath"
	"testing"
)

func TestPackageCommandWritesThemeArchive(t *testing.T) {
	root := t.TempDir()
	config := writeSiteConfig(t, root, "theme.json", "themes")
	themePath := filepath.Join(root, "themes", "example", "theme.json")
	archivePath := filepath.Join(root, "example.zip")
	writeCLIFile(t, themePath, `{"name":"example","version":"1.0.0"}`)
	writeCLIFile(t, filepath.Join(root, "themes", "example", "layouts", "default.html"), "<main>example</main>")

	if _, err := runCLI(t, "package", "--config", config, "--theme", "themes/example/theme.json", "--file", archivePath); err != nil {
		t.Fatalf("httt package error = %v", err)
	}
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}
	if !entries["theme.json"] || !entries["layouts/default.html"] {
		t.Fatalf("package entries = %#v", entries)
	}
}
