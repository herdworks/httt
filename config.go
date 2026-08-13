package httt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/herdworks/httt/files"
	"gopkg.in/yaml.v3"
)

// Config is the theme-tooling subset of a HyperTexting site config.
// Unrelated site fields are deliberately ignored so httt does not depend on htxt.
type Config struct {
	Path   string
	Theme  string `json:"theme" yaml:"theme"`
	Themes string `json:"themes_dir" yaml:"themes_dir"`
}

// LoadConfig reads the theme-tooling subset of a JSON or YAML site config.
func LoadConfig(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var source struct {
		Config struct {
			Theme  string `json:"theme" yaml:"theme"`
			Themes string `json:"themes_dir" yaml:"themes_dir"`
		} `json:"config" yaml:"config"`
	}
	switch filepath.Ext(path) {
	case ".json":
		err = json.Unmarshal(content, &source)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(content, &source)
	default:
		return Config{}, fmt.Errorf("unsupported site config type %q", filepath.Ext(path))
	}
	if err != nil {
		return Config{}, fmt.Errorf("parse site config: %w", err)
	}
	config := Config{Path: path, Theme: source.Config.Theme, Themes: source.Config.Themes}
	if config.Theme == "" {
		config.Theme = "theme.json"
	}
	if config.Themes == "" {
		config.Themes = "themes"
	}
	if err := files.ValidateRelativePath("site.config.theme", config.Theme); err != nil {
		return Config{}, err
	}
	if err := files.ValidateRelativePath("site.config.themes_dir", config.Themes); err != nil {
		return Config{}, err
	}
	return config, nil
}

// RootDir returns the directory containing the source site configuration.
func (config Config) RootDir() string { return filepath.Dir(config.Path) }

// ThemePath returns the configured theme file path rooted at the site directory.
func (config Config) ThemePath() string {
	return filepath.Join(config.RootDir(), config.Theme)
}

// ThemesDirPath returns the configured themes directory rooted at the site directory.
func (config Config) ThemesDirPath() string {
	return filepath.Join(config.RootDir(), config.Themes)
}
