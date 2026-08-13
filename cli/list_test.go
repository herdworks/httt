package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestListCommandListsConfiguredAndInstalledThemes(t *testing.T) {
	root := t.TempDir()
	config := writeSiteConfig(t, root, "theme.json", "themes")
	writeCLIFile(t, filepath.Join(root, "theme.json"), `{"name":"configured","version":"1.0.0"}`)
	writeCLIFile(t, filepath.Join(root, "themes", "installed", "2.0.0", "theme.json"), `{"name":"installed","version":"2.0.0"}`)

	output, err := runCLI(t, "ls", "--config", config)
	if err != nil {
		t.Fatalf("httt ls error = %v", err)
	}
	for _, value := range []string{"configured", "installed", "2.0.0"} {
		if !strings.Contains(output, value) {
			t.Fatalf("httt ls output missing %q:\n%s", value, output)
		}
	}
}
