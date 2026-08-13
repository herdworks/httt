package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	httheme "github.com/herdworks/httt/theme"
)

func TestInitializeThemeCopiesBootstrapAndDoesNotOverwriteTheme(t *testing.T) {
	target := filepath.Join(t.TempDir(), "theme")
	var output bytes.Buffer
	if err := initializeTheme(target, "example", &output); err != nil {
		t.Fatalf("initializeTheme() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(target, "theme.json"))
	if err != nil {
		t.Fatal(err)
	}
	var theme httheme.Theme
	if err := json.Unmarshal(content, &theme); err != nil {
		t.Fatalf("parse theme.json: %v", err)
	}
	if theme.Name != "example" || theme.Version != "0.1.0" {
		t.Fatalf("theme = %#v", theme)
	}
	if theme.Config.LayoutsDirectory != "layouts" || theme.Config.FragmentsDirectory != "fragments" || theme.Config.PluginsDirectory != "plugins" {
		t.Fatalf("theme config = %#v", theme.Config)
	}
	if _, err := os.Stat(filepath.Join(target, "layouts", "default.html")); err != nil {
		t.Fatal(err)
	}
	if err := initializeTheme(target, "example", &output); err == nil {
		t.Fatal("initializeTheme() overwrote theme.json")
	}
}

func TestDefaultThemeName(t *testing.T) {
	if got := defaultThemeName(filepath.Join("/tmp", "example-theme")); got != "example-theme" {
		t.Fatalf("defaultThemeName() = %q", got)
	}
}

func TestInitCommandDestinations(t *testing.T) {
	tests := []struct {
		name      string
		arguments []string
		setup     func(t *testing.T, root string) string
		want      func(root string) string
	}{
		{
			name:      "explicit directory",
			arguments: []string{"init", "custom", "--name", "explicit"},
			setup:     func(_ *testing.T, root string) string { return root },
			want:      func(root string) string { return filepath.Join(root, "custom") },
		},
		{
			name:      "configured themes directory",
			arguments: []string{"init", "--name", "configured"},
			setup: func(t *testing.T, root string) string {
				writeSiteConfig(t, root, "theme.json", "designs")
				return root
			},
			want: func(root string) string { return filepath.Join(root, "designs", "configured") },
		},
		{
			name:      "working directory fallback",
			arguments: []string{"init"},
			setup:     func(_ *testing.T, root string) string { return root },
			want:      func(root string) string { return root },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			withWorkingDirectory(t, test.setup(t, root))
			if _, err := runCLI(t, test.arguments...); err != nil {
				t.Fatalf("httt %s error = %v", strings.Join(test.arguments, " "), err)
			}
			if _, err := os.Stat(filepath.Join(test.want(root), "theme.json")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInitCommandDoesNotOverwriteExistingThemeOrFiles(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "theme")
	writeCLIFile(t, filepath.Join(target, "notes.txt"), "keep")
	withWorkingDirectory(t, root)
	if _, err := runCLI(t, "init", "theme", "--name", "example"); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(filepath.Join(target, "notes.txt")); err != nil || string(content) != "keep" {
		t.Fatalf("unrelated file = %q, error = %v", content, err)
	}
	if _, err := runCLI(t, "init", "theme", "--name", "example"); err == nil {
		t.Fatal("httt init overwrote existing theme")
	}
}

func TestInitCommandRejectsInvalidNameAndExtraDirectory(t *testing.T) {
	root := t.TempDir()
	withWorkingDirectory(t, root)
	if _, err := runCLI(t, "init", "--name", "nested/theme"); err == nil {
		t.Fatal("httt init accepted an unsafe name")
	}
	if _, err := runCLI(t, "init", "one", "two"); err == nil {
		t.Fatal("httt init accepted multiple directories")
	}
}
