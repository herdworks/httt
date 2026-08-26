// Package cli composes HyperTemplates' importable command-line interface.
package cli

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt"
	"github.com/herdworks/httt/files"
	httheme "github.com/herdworks/httt/theme"
)

// CLI is httt's canonical standalone command composition.
var CLI = clic.CLI{
	Command: Command(),
	Version: httt.LibVersion,
}

func Command() clic.Command {
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "The HyperTemplates CLI.",
			EnvPrefix:   "HTTT_",
		},
		Spec: clic.CommandSpec{
			Key:  "httt",
			Help: clic.DefaultHelp,
		},
		Commands: clic.MustCommandsFrom(
			InitCommand,
			ListCommand,
			InstallCommand,
			PackageCommand,
			MigrateCommand,
			RenderCommand,
			CheckCommand,
		),
	}
}

// global options

func GlobalConfigOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "config",
			ShortName:   "c",
			EnvName:     "{{ .EnvPrefix }}CONFIG",
			Description: "Path to the website configuration file.",
			Default:     "site.yaml",
		},
		Spec: clic.OptionSpec{
			Key:  "config",
			Kind: clic.StringOption,
		},
	}
}

func GlobalThemeOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "theme",
			ShortName:   "t",
			EnvName:     "{{ .EnvPrefix }}THEME",
			Description: "Path to the theme configuration file.",
		},
		Spec: clic.OptionSpec{
			Key:      "theme",
			Kind:     clic.StringOption,
			Required: true,
		},
	}
}

func GlobalFormatOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "format",
			ShortName:   "f",
			Description: "Output format: columns or json.",
			Default:     string(clic.ColumnarFormat),
		},
		Spec: clic.OptionSpec{
			Key:  "format",
			Kind: clic.StringOption,
		},
	}
}

// global helper functions

func expectedArguments(count int) func(*clic.Execution) error {
	return func(execution *clic.Execution) error {
		if execution.Command.PositionalArguments() != count {
			return fmt.Errorf("%s expected %d argument(s), got %d", execution.Command.Meta.Name, count, execution.Command.PositionalArguments())
		}
		return nil
	}
}

func validatePathSegment(field, value string) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\\`) {
		return fmt.Errorf("invalid %s %q", field, value)
	}
	return files.ValidateRelativePath(field, value)
}

func loadTheme(path string) (*httheme.Theme, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	theme := &httheme.Theme{URI: &url.URL{Path: path}}
	if err := theme.Load(io.Discard); err != nil {
		return nil, err
	}
	theme.ResolveConfig()
	if err := theme.Validate(); err != nil {
		return nil, err
	}
	return theme, nil
}
