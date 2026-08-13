package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt"
	"github.com/herdworks/httt/bootstrap"
	httheme "github.com/herdworks/httt/theme"
)

func InitCommand() clic.Command {
	name := InitNameOption()
	config := GlobalConfigOption()
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Initialize a new HyperTemplates theme.",
			Usage: []string{
				"{{ .Command }} [directory] [--name name] [--config site.yaml]",
			},
		},
		Spec: clic.CommandSpec{
			Key:  "init",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				config.Spec.Key: &config,
				name.Spec.Key:   &name,
			},
			Validator: initValidator,
			Handler:   initHandler,
		},
	}
}

func InitNameOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "name",
			Description: "Theme name and inferred destination directory.",
		},
		Spec: clic.OptionSpec{
			Key:  "name",
			Kind: clic.StringOption,
		},
	}
}

func initValidator(execution *clic.Execution) error {
	if execution.Command.PositionalArguments() > 1 {
		return fmt.Errorf("init accepts at most one directory")
	}
	if name := execution.Options["name"].StringValue(); name != "" {
		if err := validateThemeName(name); err != nil {
			return err
		}
	}
	return nil
}

func initHandler(execution *clic.Execution) error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}
	name := execution.Options["name"].StringValue()
	if name == "" {
		name = defaultThemeName(workingDirectory)
	}
	target := ""
	if execution.Command.PositionalArguments() == 1 {
		target = execution.Command.State.Arguments[0]
		if !filepath.IsAbs(target) {
			target = filepath.Join(workingDirectory, target)
		}
	} else {
		config, configErr := httt.LoadConfig(execution.Options["config"].StringValue())
		if configErr == nil {
			target = filepath.Join(config.ThemesDirPath(), name)
		} else if os.IsNotExist(configErr) {
			target = workingDirectory
		} else {
			return configErr
		}
	}
	return initializeTheme(target, name, execution.Stdout)
}

func defaultThemeName(workingDirectory string) string {
	name := filepath.Base(filepath.Clean(workingDirectory))
	if validateThemeName(name) != nil {
		return "default"
	}
	return name
}

func validateThemeName(name string) error {
	return validatePathSegment("theme name", name)
}

func initializeTheme(target, name string, output io.Writer) error {
	themePath := filepath.Join(target, "theme.json")
	if _, err := os.Stat(themePath); err == nil {
		return fmt.Errorf("theme already exists: %s", themePath)
	} else if !os.IsNotExist(err) {
		return err
	}
	themeFile, err := json.MarshalIndent(httheme.Theme{
		Name:        name,
		Description: "A HyperTemplates theme.",
		Version:     "0.1.0",
		Config: httheme.ThemeConfig{
			LayoutsDirectory:   "layouts",
			FragmentsDirectory: "fragments",
			PluginsDirectory:   "plugins",
		},
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(themePath, append(themeFile, '\n'), 0o644); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "created %s\n", themePath); err != nil {
		return err
	}
	if err := fs.WalkDir(bootstrap.Theme, "theme", func(source string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		destination := filepath.Join(target, strings.TrimPrefix(source, "theme/"))
		if _, err := os.Stat(destination); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		content, err := fs.ReadFile(bootstrap.Theme, source)
		if err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		if err = os.WriteFile(destination, content, 0o644); err != nil {
			return err
		}
		_, err = fmt.Fprintf(output, "created %s\n", destination)
		return err
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "initialized theme %q\n", target)
	return err
}
