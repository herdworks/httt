package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herdworks/clic"
)

func runCLI(t *testing.T, arguments ...string) (string, error) {
	t.Helper()
	var stdout bytes.Buffer
	result := CLI.Run(
		clic.Execution{
			Context: context.Background(),
			Stderr:  &bytes.Buffer{},
			Stdout:  &stdout,
		},
		arguments,
	)
	return stdout.String(), result.Error
}

func writeCLIFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeSiteConfig(t *testing.T, root, theme, themes string) string {
	t.Helper()
	path := filepath.Join(root, "site.yaml")
	writeCLIFile(t, path, "config:\n  theme: "+theme+"\n  themes_dir: "+themes+"\n")
	return path
}

func withWorkingDirectory(t *testing.T, directory string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	})
}

func TestCLICommandMetadata(t *testing.T) {
	if err := CLI.Validate(); err != nil {
		t.Fatalf("CLI.Validate() error = %v", err)
	}
	for _, name := range []string{"init", "ls", "install", "package", "migrate", "render", "check"} {
		if _, ok := CLI.Command.SubCommand(name); !ok {
			t.Fatalf("CLI does not contain %q", name)
		}
	}
	config := GlobalConfigOption()
	if CLI.Command.Meta.EnvPrefix != "HTTT_" || config.Meta.EnvName != "{{ .EnvPrefix }}CONFIG" || config.Meta.Default != "site.yaml" {
		t.Fatalf("config option = %#v", config.Meta)
	}
	theme := GlobalThemeOption()
	if theme.Meta.EnvName != "{{ .EnvPrefix }}THEME" || !theme.Spec.Required {
		t.Fatalf("theme option = %#v", theme)
	}
}

func TestRenderUsesThemeEnvironment(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	dataPath := filepath.Join(root, "data.json")
	writeCLIFile(t, themePath, `{}`)
	writeCLIFile(t, dataPath, `{"message":"from environment"}`)
	writeCLIFile(t, filepath.Join(root, "fragments", "notice.html"), `<notice ht-apply>${ message }</notice>`)

	var stdout bytes.Buffer
	result := CLI.Run(
		clic.Execution{
			Context: context.Background(),
			LookupEnv: func(key string) (string, bool) {
				return themePath, key == "HTTT_THEME"
			},
			Stderr: &bytes.Buffer{},
			Stdout: &stdout,
		},
		clic.Arguments{
			"render",
			"--data", dataPath,
			"--fragment", "notice",
		},
	)
	if result.Error != nil {
		t.Fatalf("httt render error = %v", result.Error)
	}
	if !strings.Contains(stdout.String(), "from environment") {
		t.Fatalf("httt render output = %q", stdout.String())
	}
}
