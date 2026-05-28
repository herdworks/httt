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

	htvalue "github.com/herdworks/hypertemplates-go/value"
)

type Theme struct {
	URI          *url.URL        `json:"-" yaml:"-"`
	Href         string          `json:"href,omitempty" yaml:"href,omitempty"`
	Name         string          `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string          `json:"description,omitempty" yaml:"description,omitempty"`
	Version      string          `json:"version,omitempty" yaml:"version,omitempty"`
	Config       ThemeConfig     `json:"config,omitempty" yaml:"config,omitempty"`
	TemplateData TemplateData    `json:"data,omitempty" yaml:"data,omitempty"`
	Assets       []string        `json:"-" yaml:"-"` // paths relative to StaticDirName()
	Plugins      htvalue.Plugins `json:"-" yaml:"-"`
}

type ThemeConfig struct {
	URL                   string `json:"url,omitempty" yaml:"url,omitempty"`
	ContentTypesDirectory string `json:"contenttypes_dir,omitempty" yaml:"contenttypes_dir,omitempty"`
	BlocksDirectory       string `json:"blocks_dir,omitempty" yaml:"blocks_dir,omitempty"`
	LayoutsDirectory      string `json:"layouts_dir,omitempty" yaml:"layouts_dir,omitempty"`
	FragmentsDirectory    string `json:"fragments_dir,omitempty" yaml:"fragments_dir,omitempty"`
	PluginsDirectory      string `json:"plugins_dir,omitempty" yaml:"plugins_dir,omitempty"`
	DataDirectory         string `json:"data_dir,omitempty" yaml:"data_dir,omitempty"`
	StaticDirectory       string `json:"static_dir,omitempty" yaml:"static_dir,omitempty"`
}

func (theme *Theme) Parse(w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

	var fileBytes []byte
	if fileBytes, err = os.ReadFile(theme.URI.Path); err == nil {
		return json.Unmarshal(fileBytes, &theme)
	} else {
		fmt.Fprintf(w, "theme \"%s\" warning: %v\n", theme.URI.Path, err)
	}
	return nil
}

func (theme *Theme) Validate() error {
	if theme.URI == nil {
		return fmt.Errorf("website.theme.uri is required")
	}
	if err := theme.Config.Validate(); err != nil {
		return err
	}
	return nil
}

func (config *ThemeConfig) Validate() error {
	fields := []struct {
		name string
		path string
	}{
		{name: "theme.config.contenttypes_dir", path: config.ContentTypesDirectory},
		{name: "theme.config.blocks_dir", path: config.BlocksDirectory},
		{name: "theme.config.layouts_dir", path: config.LayoutsDirectory},
		{name: "theme.config.fragments_dir", path: config.FragmentsDirectory},
		{name: "theme.config.plugins_dir", path: config.PluginsDirectory},
		{name: "theme.config.data_dir", path: config.DataDirectory},
		{name: "theme.config.static_dir", path: config.StaticDirectory},
	}
	for _, field := range fields {
		if err := ValidateRelativeFilePath(field.name, field.path); err != nil {
			return err
		}
	}
	return nil
}

func (theme *Theme) RootDir() string {
	return filepath.Dir(theme.URI.Path)
}

func (theme *Theme) ContentTypesDirName() string {
	return ResourceDirectoryName(theme.Config.ContentTypesDirectory, "types")
}

func (theme *Theme) ContentTypesDirPath() string {
	return filepath.Join(theme.RootDir(), theme.ContentTypesDirName())
}

func (theme *Theme) BlocksDirName() string {
	return ResourceDirectoryName(theme.Config.BlocksDirectory, "blocks")
}

func (theme *Theme) BlocksDirPath() string {
	return filepath.Join(theme.RootDir(), theme.BlocksDirName())
}

func (theme *Theme) LayoutsDirName() string {
	return ResourceDirectoryName(theme.Config.LayoutsDirectory, "layouts")
}

func (theme *Theme) LayoutsDirPath() string {
	return filepath.Join(theme.RootDir(), theme.LayoutsDirName())
}

func (theme *Theme) FragmentsDirName() string {
	return ResourceDirectoryName(theme.Config.FragmentsDirectory, "fragments")
}

func (theme *Theme) FragmentsDirPath() string {
	return filepath.Join(theme.RootDir(), theme.FragmentsDirName())
}

func (theme *Theme) PluginsDirName() string {
	return ResourceDirectoryName(theme.Config.PluginsDirectory, "plugins")
}

func (theme *Theme) PluginsDirPath() string {
	return filepath.Join(theme.RootDir(), theme.PluginsDirName())
}

func (theme *Theme) StaticDirName() string {
	return ResourceDirectoryName(theme.Config.StaticDirectory, "static")
}

func (theme *Theme) StaticDirPath() string {
	return filepath.Join(theme.RootDir(), theme.StaticDirName())
}

func (theme *Theme) DataDirName() string {
	return ResourceDirectoryName(theme.Config.DataDirectory, "data")
}

func (theme *Theme) DataDirPath() string {
	return filepath.Join(theme.RootDir(), theme.DataDirName())
}

func (theme *Theme) Directories() []string {
	return []string{
		theme.ContentTypesDirPath(),
		theme.BlocksDirPath(),
		theme.LayoutsDirPath(),
		theme.FragmentsDirPath(),
		theme.PluginsDirPath(),
		theme.DataDirPath(),
		theme.StaticDirPath(),
	}
}

func (theme *Theme) ResolveConfig() {
	if len(theme.Name) == 0 {
		theme.Name = "Untitled Theme"
	}
	if len(theme.Version) == 0 {
		theme.Version = "0.0.0"
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
	resourceRef = filepath.ToSlash(resourceRef)
	if filepath.Ext(resourceRef) == "" {
		resourceRef += ".html"
	}
	if _, err := CleanRelativeFilePath(resourceRef); err != nil {
		return nil, err
	}
	if filepath.Ext(resourceRef) != ".html" {
		return nil, fmt.Errorf("template resource \"%s\" is not an HTML file", resourceRef)
	}
	resourcePath := theme.ResourcePath(resourceRef)
	exists, err := theme.ResourceExists(resourcePath)
	if err != nil {
		return nil, err
	}
	if exists {
		resourceBytes, err := os.ReadFile(resourcePath)
		if err != nil {
			return nil, err
		}
		return html.Parse(bytes.NewReader(resourceBytes))
	}
	// On disk-miss, fall back to embedded builtins at the same relative path.
	builtinBytes, found, err := getBuiltinResource(resourceRef)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("template resource \"%s\" not found", resourcePath)
	}
	return html.Parse(bytes.NewReader(builtinBytes))
}
func (theme *Theme) LayoutHTML(layoutRef string) (*html.Node, error) {
	layoutDir := theme.LayoutsDirName()
	layoutRef = filepath.ToSlash(layoutRef)
	layoutRef = strings.TrimPrefix(layoutRef, "/")
	if !strings.HasPrefix(layoutRef, layoutDir+"/") {
		layoutRef = path.Join(layoutDir, layoutRef)
	}
	return theme.ResourceHTML(layoutRef)
}

func (theme *Theme) DiscoverAssets() error {
	if err := theme.Config.Validate(); err != nil {
		return err
	}
	assets, err := ListFiles(theme.StaticDirPath(), true)
	if err != nil {
		return err
	}
	theme.Assets = assets
	return nil
}

func (theme *Theme) DiscoverPlugins() error {
	if err := theme.Config.Validate(); err != nil {
		return err
	}
	pluginPaths, err := ListFiles(theme.PluginsDirPath(), true)
	if err != nil {
		return err
	}
	paths, err := LocalFilePaths(theme.PluginsDirPath(), pluginPaths)
	if err != nil {
		return err
	}
	theme.Plugins, err = htvalue.LoadPlugins(paths)
	if err != nil {
		return err
	}
	return nil
}

// FilePaths lists all theme files with paths relative to the theme root directory.
func (theme *Theme) FilePaths() (paths []string, err error) {
	var root string
	var rootAbs string
	var seen map[string]bool = map[string]bool{}

	if err = theme.Validate(); err != nil {
		return nil, err
	}

	root = theme.RootDir()
	if rootAbs, err = filepath.Abs(root); err != nil {
		return nil, fmt.Errorf("resolve theme root %q: %w", root, err)
	}
	if absConfigPath, err := filepath.Abs(theme.URI.Path); err == nil {
		if relConfigPath, err := filepath.Rel(rootAbs, absConfigPath); err == nil {
			relConfigPath = filepath.ToSlash(relConfigPath)
			seen[relConfigPath] = true
			paths = append(paths, relConfigPath)
		}
	}
	for _, dirPath := range theme.Directories() {
		dir, err := RelativeFilePath(rootAbs, dirPath)
		if err != nil {
			return nil, err
		}
		dir = filepath.ToSlash(dir)
		var files []string
		if files, err = ListFiles(dirPath, true); err != nil {
			return nil, err
		}
		for _, p := range files {
			var relPath string

			relPath = filepath.ToSlash(filepath.Join(dir, p))
			if relPath == "." || relPath == "" {
				continue
			}
			if relPath == ".." || strings.HasPrefix(relPath, "../") {
				return nil, fmt.Errorf("theme file path %q escapes root %q", p, rootAbs)
			}
			if _, ok := seen[relPath]; ok {
				continue
			}
			seen[relPath] = true
			paths = append(paths, relPath)
		}
	}
	return paths, nil
}

// Archive writes a zip archive containing every file returned by theme.FilePaths().
func (theme *Theme) Archive(archive string, w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

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

	// Create empty directory entries for all configured theme directories,
	// ensuring they are present in the archive even if they contain no files.
	for _, dirPath := range theme.Directories() {
		dir, err := RelativeFilePath(root, dirPath)
		if err != nil {
			return err
		}
		dir = filepath.ToSlash(dir)
		var dirHeader *zip.FileHeader = &zip.FileHeader{
			Name:   filepath.ToSlash(dir) + "/",
			Method: zip.Store,
		}
		dirHeader.SetMode(os.ModeDir | 0755)
		if _, err = zipper.CreateHeader(dirHeader); err != nil {
			return fmt.Errorf("create directory entry %q: %w", dirHeader.Name, err)
		}
	}

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

		fmt.Fprintf(w, "file %q writing\n", header.Name)

		if written, err = io.Copy(entryWriter, sourceFile); err != nil {
			sourceFile.Close()
			return fmt.Errorf("write zip entry %q: %w", header.Name, err)
		}

		if err = sourceFile.Close(); err != nil {
			return fmt.Errorf("close theme file %q: %w", sourcePath, err)
		}

		fmt.Fprintf(w, "file %q written (%d bytes)\n", header.Name, written)
	}

	return nil
}
