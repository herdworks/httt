package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herdworks/clic"
	httheme "github.com/herdworks/httt/theme"
)

func TestReleaseCommandStandaloneAndConsumerMount(t *testing.T) {
	for _, mount := range []string{"standalone", "consumer"} {
		t.Run(mount, func(t *testing.T) {
			root := t.TempDir()
			config := filepath.Join(root, "site.yaml")
			writeCLIFile(t, config, "config:\n  theme: theme.json\n  static_dir: public\n")
			writeCLIFile(t, filepath.Join(root, "theme.json"), `{"name":"Site Theme","version":"1.2.3"}`)
			writeCLIFile(t, filepath.Join(root, "static", "site.css"), "body{}")
			command := Command()
			args := clic.Arguments{"theme", "release"}
			prefix := "HTTT_"
			if mount == "consumer" {
				// This is hyperctl's existing mount: change only root metadata.
				command.Spec.Key = "theme"
				command.Meta.EnvPrefix = ""
				mounted := command
				command = clic.Command{
					Meta: clic.CommandMeta{
						EnvPrefix: "HYPERCTL_",
					},
					Spec: clic.CommandSpec{
						Key: "hyperctl",
					},
					Commands: clic.Commands{
						"theme": &mounted,
					},
				}
				prefix = "HYPERCTL_"
			}
			app := clic.CLI{Command: command}
			var stdout bytes.Buffer
			result := app.Run(clic.Execution{
				Context: context.Background(),
				Stdout:  &stdout,
				Stderr:  &bytes.Buffer{},
				LookupEnv: func(key string) (string, bool) {
					switch key {
					case prefix + "CONFIG":
						return config, true
					case prefix + "THEME":
						return "theme.json", true
					}
					return "", false
				},
			}, args)
			if result.Error != nil {
				t.Fatal(result.Error)
			}
			indexPath := filepath.Join(root, "public", ".hypertexting", "themes.json")
			data, err := os.ReadFile(indexPath)
			if err != nil {
				t.Fatal(err)
			}
			var index httheme.ReleaseIndex
			if err := json.Unmarshal(data, &index); err != nil {
				t.Fatal(err)
			}
			if index.Name != "Site Theme" || index.Latest != "1.2.3" {
				t.Fatalf("index = %#v", index)
			}
			if _, err := os.Stat(filepath.Join(root, "public", ".hypertexting", "themes", "site-theme-v1.2.3.zip")); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), "theme release prepared") {
				t.Fatalf("stdout = %q", stdout.String())
			}
		})
	}
}

func TestReleaseCLIRejectsMissingMetadataInsteadOfDisplayDefaults(t *testing.T) {
	root := t.TempDir()
	config := writeSiteConfig(t, root, "theme.json", "themes")
	writeCLIFile(t, filepath.Join(root, "theme.json"), `{}`)
	if _, err := runCLI(t, "theme", "release", "-c", config, "-t", "theme.json"); err == nil {
		t.Fatal("published display defaults")
	}
	if _, err := os.Stat(filepath.Join(root, "static", ".hypertexting", "themes.json")); !os.IsNotExist(err) {
		t.Fatalf("index: %v", err)
	}
}

func TestReleaseCLIUsesRelativeDefaultSiteAndCustomArchives(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeSiteConfig(t, root, "theme.json", "themes")
	writeCLIFile(t, "theme.json", `{"name":"Overlap","version":"1.0.0"}`)
	writeCLIFile(t, "static/site.css", "body{}")
	for _, command := range [][]string{{"release"}, {"theme", "release"}} {
		args := append(append([]string{}, command...), "-t", "theme.json", "-f", "downloads/overlap.zip")
		if _, err := runCLI(t, args...); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat("static/downloads/overlap.zip"); err != nil {
		t.Fatal(err)
	}
}

func TestPackageCLICompatibilityAndThemeSpelling(t *testing.T) {
	root := t.TempDir()
	config := writeSiteConfig(t, root, "theme.json", "themes")
	writeCLIFile(t, filepath.Join(root, "theme.json"), `{"name":"example","version":"1.0.0"}`)
	var previous []byte
	for _, command := range [][]string{{"package"}, {"theme", "package"}} {
		archive := filepath.Join(root, strings.Join(command, "-")+".zip")
		args := append(append([]string{}, command...), "-c", config, "-t", "theme.json", "-f", archive)
		if _, err := runCLI(t, args...); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(archive)
		if err != nil {
			t.Fatal(err)
		}
		if previous != nil && !bytes.Equal(previous, data) {
			t.Fatal("theme package differs from top-level package")
		}
		previous = data
	}
}

func TestThemeReleaseAndPackageMountedHelp(t *testing.T) {
	for _, command := range [][]string{{"theme", "release"}, {"theme", "package"}, {"release"}, {"package"}} {
		args := append(append([]string{}, command...), "--help")
		output, err := runCLI(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output, "httt "+strings.Join(command, " ")) {
			t.Fatalf("mounted help = %q", output)
		}
	}
}
