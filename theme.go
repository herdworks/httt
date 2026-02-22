package hypertemplates

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

type Theme struct {
	URI         *url.URL         `json:"-" yaml:"-"`
	Href        string           `json:"href,omitempty" yaml:"href,omitempty"`
	Name        string           `json:"name,omitempty" yaml:"name,omitempty"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string           `json:"version,omitempty" yaml:"version,omitempty"`
	Config      ThemeConfig `json:"config,omitempty" yaml:"config,omitempty"`
}

type ThemeConfig struct {
	URL                   string `json:"url,omitempty" yaml:"url,omitempty"`
	ContentTypesDirectory string `json:"contenttypes_dir,omitempty" yaml:"contenttypes_dir,omitempty"`
	BlocksDirectory       string `json:"blocks_dir,omitempty" yaml:"blocks_dir,omitempty"`
	LayoutsDirectory      string `json:"layouts_dir,omitempty" yaml:"layouts_dir,omitempty"`
	FragmentsDirectory    string `json:"fragments_dir,omitempty" yaml:"fragments_dir,omitempty"`
	StaticDirectory       string `json:"static_dir,omitempty" yaml:"static_dir,omitempty"`
}

func (theme *Theme) Parse() (err error) {
	var fileBytes []byte
	if fileBytes, err = os.ReadFile(theme.URI.Path); err == nil {
		return json.Unmarshal(fileBytes, &theme)
	} else {
		fmt.Printf("theme \"%s\" warning: %v\n", theme.URI.Path, err)
	}
	return nil
}

func (theme *Theme) Validate() error {
	if theme.URI == nil {
		return fmt.Errorf("website.theme.uri is required")
	}
	return nil
}

func (theme *Theme) RootDir() string {
	return filepath.Dir(theme.URI.Path)
}

func (theme *Theme) ContentTypesDir() string {
	if len(theme.Config.ContentTypesDirectory) == 0 {
		theme.Config.ContentTypesDirectory = "types"
	}
	return filepath.Join(theme.RootDir(), theme.Config.ContentTypesDirectory)
}

func (theme *Theme) ElementsDir() string {
	if len(theme.Config.BlocksDirectory) == 0 {
		theme.Config.BlocksDirectory = "blocks"
	}
	return filepath.Join(theme.RootDir(), theme.Config.BlocksDirectory)
}

func (theme *Theme) LayoutsDir() string {
	if len(theme.Config.LayoutsDirectory) == 0 {
		theme.Config.LayoutsDirectory = "layouts"
	}
	return filepath.Join(theme.RootDir(), theme.Config.LayoutsDirectory)
}

func (theme *Theme) StaticDir() string {
	if len(theme.Config.StaticDirectory) == 0 {
		theme.Config.StaticDirectory = "static"
	}
	return filepath.Join(theme.RootDir(), theme.Config.StaticDirectory)
}

func (theme *Theme) ResolveConfig() {
	if len(theme.Name) == 0 {
		theme.Name = "Untitled Theme"
	}
	if len(theme.Version) == 0 {
		theme.Version = "0.0.0"
	}
	if len(theme.Config.ContentTypesDirectory) == 0 {
		theme.Config.ContentTypesDirectory = "types"
	}
	if len(theme.Config.BlocksDirectory) == 0 {
		theme.Config.BlocksDirectory = "blocks"
	}
	if len(theme.Config.LayoutsDirectory) == 0 {
		theme.Config.LayoutsDirectory = "layouts"
	}
	if len(theme.Config.FragmentsDirectory) == 0 {
		theme.Config.FragmentsDirectory = "fragments"
	}
	if len(theme.Config.StaticDirectory) == 0 {
		theme.Config.StaticDirectory = "static"
	}
}

// ResourcePath returns a path for a resource within the theme.
//
// Usage:
//
//	theme.ResourcePath("layouts/default")
//	theme.ResourcePath("layouts/default.html")
func (theme *Theme) ResourcePath(resourceRef string) string {
	if filepath.Ext(resourceRef) == "" {
		resourceRef += ".html"
	}
	return filepath.Join(theme.RootDir(), resourceRef)
}
func (theme *Theme) ResourceExists(path string) (status bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ResourceHTML returns the contents of a resource within the theme.
//
// Usage:
//
//	theme.ResourceHTML("layouts/default")
//	theme.ResourceHTML("layouts/default.html")
func (theme *Theme) ResourceHTML(resourceRef string) (*html.Node, error) {
	if filepath.Ext(resourceRef) == "" {
		resourceRef += ".html"
	}
	if filepath.Ext(resourceRef) != ".html" {
		return nil, fmt.Errorf("template resource \"%s\" is not an HTML file", resourceRef)
	}
	resourcePath := theme.ResourcePath(resourceRef)
	exists, err := theme.ResourceExists(resourcePath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("template resource \"%s\" not found", resourcePath)
	}
	resourceBytes, err := os.ReadFile(resourcePath)
	if err != nil {
		return nil, err
	}
	return html.Parse(bytes.NewReader(resourceBytes))
}
func (theme *Theme) LayoutHTML(layoutRef string) (*html.Node, error) {
	if len(theme.Config.LayoutsDirectory) == 0 {
		theme.ResolveConfig()
	}
	var layoutPrefix = theme.Config.LayoutsDirectory
	if !strings.HasPrefix(layoutRef, layoutPrefix) {
		layoutRef = filepath.Join(layoutPrefix, layoutRef)
	}
	return theme.ResourceHTML(layoutRef)
}

func (theme *Theme) Assets() (assets HyperFiles) {
	return GetFiles(theme.StaticDir(), true)
}

