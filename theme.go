package hypertemplates

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

type Theme struct {
	URI          *url.URL     `json:"-" yaml:"-"`
	Href         string       `json:"href,omitempty" yaml:"href,omitempty"`
	Name         string       `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string       `json:"description,omitempty" yaml:"description,omitempty"`
	Version      string       `json:"version,omitempty" yaml:"version,omitempty"`
	Config       ThemeConfig  `json:"config,omitempty" yaml:"config,omitempty"`
	TemplateData TemplateData `json:"data,omitempty" yaml:"data,omitempty"`
}

type ThemeConfig struct {
	URL                   string `json:"url,omitempty" yaml:"url,omitempty"`
	ContentTypesDirectory string `json:"contenttypes_dir,omitempty" yaml:"contenttypes_dir,omitempty"`
	BlocksDirectory       string `json:"blocks_dir,omitempty" yaml:"blocks_dir,omitempty"`
	LayoutsDirectory      string `json:"layouts_dir,omitempty" yaml:"layouts_dir,omitempty"`
	FragmentsDirectory    string `json:"fragments_dir,omitempty" yaml:"fragments_dir,omitempty"`
	DataDirectory         string `json:"data_dir,omitempty" yaml:"data_dir,omitempty"`
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

func (theme *Theme) BlocksDir() string {
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

func (theme *Theme) FragmentsDir() string {
	if len(theme.Config.FragmentsDirectory) == 0 {
		theme.Config.FragmentsDirectory = "fragments"
	}
	return filepath.Join(theme.RootDir(), theme.Config.FragmentsDirectory)
}

func (theme *Theme) StaticDir() string {
	if len(theme.Config.StaticDirectory) == 0 {
		theme.Config.StaticDirectory = "static"
	}
	return filepath.Join(theme.RootDir(), theme.Config.StaticDirectory)
}

func (theme *Theme) DataDir() string {
	if len(theme.Config.DataDirectory) == 0 {
		theme.Config.DataDirectory = "data"
	}
	return filepath.Join(theme.RootDir(), theme.Config.DataDirectory)
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
	if len(theme.Config.DataDirectory) == 0 {
		theme.Config.DataDirectory = "data"
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

// List all theme files with their paths relative to the theme root directory.
func (theme *Theme) FilePaths() (paths []string, err error) {
	var root string
	var rootAbs string
	var seen map[string]struct{} = map[string]struct{}{}

	if err = theme.Validate(); err != nil {
		return nil, err
	}

	theme.ResolveConfig()
	root = theme.RootDir()
	if rootAbs, err = filepath.Abs(root); err != nil {
		return nil, fmt.Errorf("resolve theme root %q: %w", root, err)
	}

	var directories []string = []string{
		theme.Config.ContentTypesDirectory,
		theme.Config.BlocksDirectory,
		theme.Config.LayoutsDirectory,
		theme.Config.FragmentsDirectory,
		theme.Config.DataDirectory,
		theme.Config.StaticDirectory,
	}
	for _, dir := range directories {
		dirPath := filepath.Join(rootAbs, dir)
		for _, p := range GetPaths(dirPath, true) {
			var absPath string
			var relPath string

			if absPath, err = filepath.Abs(p); err != nil {
				return nil, fmt.Errorf("resolve theme file path %q: %w", p, err)
			}

			if relPath, err = filepath.Rel(rootAbs, absPath); err != nil {
				return nil, fmt.Errorf("resolve relative theme file path %q: %w", p, err)
			}

			relPath = filepath.ToSlash(relPath)
			if relPath == "." || relPath == "" {
				continue
			}
			if relPath == ".." || strings.HasPrefix(relPath, "../") {
				return nil, fmt.Errorf("theme file path %q escapes root %q", p, rootAbs)
			}
			if _, ok := seen[relPath]; ok {
				continue
			}
			seen[relPath] = struct{}{}
			paths = append(paths, relPath)
		}
	}
	return paths, nil
}

// Archive uses theme.FilePaths() to list all theme files, then create a zip archive containing those files at the specified path.
func (theme *Theme) Archive(archive string) (err error) {
	var paths []string
	var archiveFile *os.File
	var zipper *zip.Writer
	var root string = theme.RootDir()

	if paths, err = theme.FilePaths(); err != nil {
		return err
	}
	if archiveFile, err = os.Create(archive); err != nil {
		return fmt.Errorf("create theme archive %q: %w", archive, err)
	}
	defer func() {
		if closeErr := archiveFile.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close theme archive %q: %w", archive, closeErr)
		}
	}()

	zipper = zip.NewWriter(archiveFile)
	defer func() {
		if closeErr := zipper.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("finalize theme archive %q: %w", archive, closeErr)
		}
	}()

	for _, relPath := range paths {
		var sourcePath string
		var sourceFile *os.File
		var sourceInfo os.FileInfo
		var header *zip.FileHeader
		var entryWriter io.Writer
		var written int64

		relPath = path.Clean(strings.TrimPrefix(filepath.ToSlash(relPath), "./"))
		if relPath == "." || relPath == "" {
			continue
		}
		if strings.HasPrefix(relPath, "/") || relPath == ".." || strings.HasPrefix(relPath, "../") {
			return fmt.Errorf("invalid archive entry path %q", relPath)
		}

		sourcePath = filepath.Join(root, filepath.FromSlash(relPath))

		if sourceInfo, err = os.Stat(sourcePath); err != nil {
			return fmt.Errorf("stat theme file %q: %w", sourcePath, err)
		}

		if header, err = zip.FileInfoHeader(sourceInfo); err != nil {
			return fmt.Errorf("build zip header for %q: %w", sourcePath, err)
		}

		header.Name = filepath.ToSlash(relPath)
		header.Method = zip.Deflate

		if entryWriter, err = zipper.CreateHeader(header); err != nil {
			return fmt.Errorf("create zip entry %q: %w", header.Name, err)
		}

		if sourceFile, err = os.Open(sourcePath); err != nil {
			return fmt.Errorf("open theme file %q: %w", sourcePath, err)
		}

		fmt.Printf("file %q writing\n", header.Name)

		if written, err = io.Copy(entryWriter, sourceFile); err != nil {
			sourceFile.Close()
			return fmt.Errorf("write zip entry %q: %w", header.Name, err)
		}

		if err = sourceFile.Close(); err != nil {
			return fmt.Errorf("close theme file %q: %w", sourcePath, err)
		}

		fmt.Printf("file %q written (%d bytes)\n", header.Name, written)
	}

	return nil
}
