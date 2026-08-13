package httt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigResolvesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "site.yaml")
	if err := os.WriteFile(path, []byte("config:\n  themes_dir: designs\n  theme: designs/example/theme.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := config.ThemesDirPath(), filepath.Join(filepath.Dir(path), "designs"); got != want {
		t.Fatalf("ThemesDirPath() = %q, want %q", got, want)
	}
}

func TestLoadConfigSupportsJSONAndRejectsInvalidPaths(t *testing.T) {
	root := t.TempDir()
	jsonPath := filepath.Join(root, "site.json")
	if err := os.WriteFile(jsonPath, []byte(`{"config":{"theme":"themes/default/theme.json","themes_dir":"themes"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := config.ThemePath(), filepath.Join(root, "themes", "default", "theme.json"); got != want {
		t.Fatalf("ThemePath() = %q, want %q", got, want)
	}

	invalidPath := filepath.Join(root, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte("config:\n  themes_dir: ../outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err = LoadConfig(invalidPath); err == nil {
		t.Fatal("LoadConfig() accepted an escaping themes directory")
	}
}

func TestLoadConfigRejectsUnsupportedAndMalformedFiles(t *testing.T) {
	root := t.TempDir()
	unsupported := filepath.Join(root, "site.txt")
	if err := os.WriteFile(unsupported, []byte("config: {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(unsupported); err == nil {
		t.Fatal("LoadConfig() accepted an unsupported config type")
	}
	malformed := filepath.Join(root, "site.yaml")
	if err := os.WriteFile(malformed, []byte("config: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(malformed); err == nil {
		t.Fatal("LoadConfig() accepted malformed YAML")
	}
}
