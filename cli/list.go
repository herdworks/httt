package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt"
	httheme "github.com/herdworks/httt/theme"
)

func ListCommand() clic.Command {
	config := GlobalConfigOption()
	format := GlobalFormatOption()
	return clic.Command{
		Meta: clic.CommandMeta{
			Description: "List installed themes.",
			Usage: []string{
				"{{ .Command }} [--config site.yaml]",
			},
		},
		Spec: clic.CommandSpec{
			Key:  "ls",
			Help: clic.DefaultHelp,
			Options: clic.Options{
				"config": &config,
				"format": &format,
			},
			Validator: expectedArguments(0),
			Handler:   listHandler,
		},
	}
}

func listHandler(execution *clic.Execution) error {
	config, err := httt.LoadConfig(execution.Options["config"].StringValue())
	if err != nil {
		return err
	}
	themes, err := listThemes(config)
	if err != nil {
		return err
	}
	return clic.WriteList(
		execution.Stdout,
		clic.Format(execution.Options["format"].StringValue()),
		themes,
		clic.Field{Key: "path", Header: "Path"},
		clic.Field{Key: "name", Header: "Name"},
		clic.Field{Key: "version", Header: "Version"},
	)
}

func listThemes(config httt.Config) (httheme.Themes, error) {
	paths := []string{config.ThemePath()}
	err := filepath.WalkDir(config.ThemesDirPath(), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "theme.json") && filepath.Clean(path) != filepath.Clean(config.ThemePath()) {
			paths = append(paths, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	themes := httheme.Themes{}
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		theme, err := loadTheme(path)
		if err != nil {
			return nil, err
		}
		themes = append(themes, *theme)
	}
	return themes, nil
}
