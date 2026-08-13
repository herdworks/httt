package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCommandUsesConfiguredTheme(t *testing.T) {
	root := t.TempDir()
	config := writeSiteConfig(t, root, "themes/legacy/theme.json", "themes")
	writeCLIFile(t, filepath.Join(root, "themes", "legacy", "theme.json"), `{"name":"legacy","version":"1.0.0"}`)
	layoutPath := filepath.Join(root, "themes", "legacy", "layouts", "default.html")
	writeCLIFile(t, layoutPath, `<main ht-content="content">fallback</main>`)

	if _, err := runCLI(t, "migrate", "--config", config); err != nil {
		t.Fatalf("httt migrate error = %v", err)
	}
	content, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "ht-apply") || strings.Contains(string(content), "ht-content") {
		t.Fatalf("migrated layout = %s", content)
	}
}
