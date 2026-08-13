package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt"
)

func PackageCommand() clic.Command {
	config := GlobalConfigOption()
	theme := GlobalThemeOption()
	file := PackageOutputOption()
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "Package a theme into a zip archive.",
			Usage: []string{
				"{{ .Command }} -c site.yaml -t themes/default/theme.json [-f theme-v0.1.0.zip]",
			},
		},
		Spec: clic.CommandSpec{
			Key:  "package",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				config.Spec.Key: &config,
				theme.Spec.Key:  &theme,
				file.Spec.Key:   &file,
			},
			Validator: expectedArguments(0),
			Handler:   packageHandler,
		},
	}
}

func PackageOutputOption() clic.Option {
	return clic.Option{
		Meta: clic.OptionMeta{
			Name:        "file",
			ShortName:   "f",
			Description: "Output zip archive path.",
		},
		Spec: clic.OptionSpec{
			Key:  "file",
			Kind: clic.StringOption,
		},
	}
}

func packageHandler(execution *clic.Execution) error {
	config, err := httt.LoadConfig(execution.Options["config"].StringValue())
	if err != nil {
		return err
	}
	path := execution.Options["theme"].StringValue()
	if !filepath.IsAbs(path) {
		path = filepath.Join(config.RootDir(), path)
	}
	theme, err := loadTheme(path)
	if err != nil {
		return err
	}
	archive := execution.Options["file"].StringValue()
	if archive == "" {
		archive = fmt.Sprintf("%s-v%s.zip", theme.Name, theme.Version)
	}
	if _, err = os.Stat(archive); err == nil {
		return fmt.Errorf("archive already exists: %s", archive)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err = theme.Archive(archive, io.Discard); err != nil {
		return err
	}
	_, err = fmt.Fprintf(execution.Stdout, "theme packaged successfully: %s\n", archive)
	return err
}
