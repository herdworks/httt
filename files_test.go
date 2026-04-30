package hypertemplates

import (
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeTestFile(t *testing.T, path string) {
	writeTestFileContent(t, path, "ok")
}

func writeTestFileContent(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir test file dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func TestListFiles(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "b.txt"))
	writeTestFile(t, filepath.Join(root, "a.txt"))
	writeTestFile(t, filepath.Join(root, "nested", "c.txt"))

	got, err := ListFiles(root, false)
	if err != nil {
		t.Fatalf("ListFiles non-recursive returned error: %v", err)
	}
	want := []string{
		"a.txt",
		"b.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListFiles non-recursive = %#v, want %#v", got, want)
	}

	got, err = ListFiles(root, true)
	if err != nil {
		t.Fatalf("ListFiles recursive returned error: %v", err)
	}
	want = []string{
		"a.txt",
		"b.txt",
		"nested/c.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListFiles recursive = %#v, want %#v", got, want)
	}
}

func TestListFilesMissingDirectory(t *testing.T) {
	got, err := ListFiles(filepath.Join(t.TempDir(), "missing"), true)
	if err != nil {
		t.Fatalf("ListFiles missing directory returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ListFiles missing directory = %#v, want empty", got)
	}
}

func TestResourceDirectoryName(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		fallback  string
		want      string
	}{
		{name: "default", directory: "", fallback: "static", want: "static"},
		{name: "configured", directory: "assets", fallback: "static", want: "assets"},
		{name: "nested", directory: "assets/images", fallback: "static", want: "assets/images"},
		{name: "absolute", directory: "/assets", fallback: "static", want: "/assets"},
		{name: "parent traversal", directory: "../assets", fallback: "static", want: "../assets"},
		{name: "backslash", directory: `assets\images`, fallback: "static", want: `assets\images`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ResourceDirectoryName(test.directory, test.fallback)
			if got != test.want {
				t.Fatalf("ResourceDirectoryName(%q, %q) = %q, want %q", test.directory, test.fallback, got, test.want)
			}
		})
	}
}

func TestThemeConfigValidateRejectsUnsafePaths(t *testing.T) {
	tests := []struct {
		name   string
		config ThemeConfig
	}{
		{name: "content types", config: ThemeConfig{ContentTypesDirectory: "../types"}},
		{name: "blocks", config: ThemeConfig{BlocksDirectory: "/blocks"}},
		{name: "layouts", config: ThemeConfig{LayoutsDirectory: `layouts\main`}},
		{name: "fragments", config: ThemeConfig{FragmentsDirectory: "."}},
		{name: "plugins", config: ThemeConfig{PluginsDirectory: "../plugins"}},
		{name: "data", config: ThemeConfig{DataDirectory: "/data"}},
		{name: "static", config: ThemeConfig{StaticDirectory: `static\assets`}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.config.Validate(); err == nil {
				t.Fatal("ThemeConfig.Validate() error = nil")
			}
		})
	}
}

func TestThemeConfigValidateAllowsEmptyDefaults(t *testing.T) {
	var config ThemeConfig
	if err := config.Validate(); err != nil {
		t.Fatalf("ThemeConfig.Validate() error = %v", err)
	}
}

func TestListFilesByPattern(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "keep.md"))
	writeTestFile(t, filepath.Join(root, "skip.md"))
	writeTestFile(t, filepath.Join(root, "asset.jpg"))
	writeTestFile(t, filepath.Join(root, "nested", "keep.md"))

	got, err := ListFilesByPattern(root, false, `^keep\.md$`, `skip`)
	if err != nil {
		t.Fatalf("ListFilesByPattern returned error: %v", err)
	}
	want := []string{"keep.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListFilesByPattern = %#v, want %#v", got, want)
	}

	got, err = ListFilesByPattern(root, true, `^nested/.*\.md$`, "")
	if err != nil {
		t.Fatalf("ListFilesByPattern recursive returned error: %v", err)
	}
	want = []string{"nested/keep.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListFilesByPattern recursive = %#v, want %#v", got, want)
	}
}

func TestThemeDiscoverAssetsReplacesExistingAssets(t *testing.T) {
	root := t.TempDir()
	theme := &Theme{
		URI:    &url.URL{Path: filepath.Join(root, "theme.json")},
		Assets: []string{"stale.css"},
	}
	writeTestFile(t, filepath.Join(theme.StaticDirPath(), "theme.css"))

	if err := theme.DiscoverAssets(); err != nil {
		t.Fatalf("DiscoverAssets returned error: %v", err)
	}
	want := []string{"theme.css"}
	if !reflect.DeepEqual(theme.Assets, want) {
		t.Fatalf("Theme.Assets = %#v, want %#v", theme.Assets, want)
	}

	if err := os.Remove(filepath.Join(theme.StaticDirPath(), "theme.css")); err != nil {
		t.Fatalf("remove theme asset: %v", err)
	}
	writeTestFile(t, filepath.Join(theme.StaticDirPath(), "next.css"))

	if err := theme.DiscoverAssets(); err != nil {
		t.Fatalf("second DiscoverAssets returned error: %v", err)
	}
	want = []string{"next.css"}
	if !reflect.DeepEqual(theme.Assets, want) {
		t.Fatalf("Theme.Assets after rediscovery = %#v, want %#v", theme.Assets, want)
	}
}

func TestThemeDirectoryGettersDoNotMutateConfig(t *testing.T) {
	root := t.TempDir()
	theme := &Theme{
		URI: &url.URL{Path: filepath.Join(root, "theme.json")},
	}

	pathTests := map[string]string{
		"content types": theme.ContentTypesDirPath(),
		"blocks":        theme.BlocksDirPath(),
		"layouts":       theme.LayoutsDirPath(),
		"fragments":     theme.FragmentsDirPath(),
		"plugins":       theme.PluginsDirPath(),
		"static":        theme.StaticDirPath(),
		"data":          theme.DataDirPath(),
	}
	wantPaths := map[string]string{
		"content types": filepath.Join(root, "types"),
		"blocks":        filepath.Join(root, "blocks"),
		"layouts":       filepath.Join(root, "layouts"),
		"fragments":     filepath.Join(root, "fragments"),
		"plugins":       filepath.Join(root, "plugins"),
		"static":        filepath.Join(root, "static"),
		"data":          filepath.Join(root, "data"),
	}
	if !reflect.DeepEqual(pathTests, wantPaths) {
		t.Fatalf("theme directory path getters = %#v, want %#v", pathTests, wantPaths)
	}

	nameTests := map[string]string{
		"content types": theme.ContentTypesDirName(),
		"blocks":        theme.BlocksDirName(),
		"layouts":       theme.LayoutsDirName(),
		"fragments":     theme.FragmentsDirName(),
		"plugins":       theme.PluginsDirName(),
		"static":        theme.StaticDirName(),
		"data":          theme.DataDirName(),
	}
	wantNames := map[string]string{
		"content types": "types",
		"blocks":        "blocks",
		"layouts":       "layouts",
		"fragments":     "fragments",
		"plugins":       "plugins",
		"static":        "static",
		"data":          "data",
	}
	if !reflect.DeepEqual(nameTests, wantNames) {
		t.Fatalf("theme directory name getters = %#v, want %#v", nameTests, wantNames)
	}
	if theme.Config != (ThemeConfig{}) {
		t.Fatalf("theme directory getters mutated config: %#v", theme.Config)
	}
}

func TestThemeDirectories(t *testing.T) {
	root := t.TempDir()
	theme := &Theme{
		URI: &url.URL{Path: filepath.Join(root, "theme.json")},
		Config: ThemeConfig{
			LayoutsDirectory: "templates",
			StaticDirectory:  "assets",
		},
	}

	got := theme.Directories()
	want := []string{
		filepath.Join(root, "types"),
		filepath.Join(root, "blocks"),
		filepath.Join(root, "templates"),
		filepath.Join(root, "fragments"),
		filepath.Join(root, "plugins"),
		filepath.Join(root, "data"),
		filepath.Join(root, "assets"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Theme.Directories = %#v, want %#v", got, want)
	}
}

func TestThemeResolveConfigDoesNotMutateDirectoryConfig(t *testing.T) {
	theme := &Theme{}

	theme.ResolveConfig()

	if theme.Name != "Untitled Theme" {
		t.Fatalf("Theme.Name = %q, want Untitled Theme", theme.Name)
	}
	if theme.Version != "0.0.0" {
		t.Fatalf("Theme.Version = %q, want 0.0.0", theme.Version)
	}
	if theme.Config != (ThemeConfig{}) {
		t.Fatalf("ResolveConfig mutated directory config: %#v", theme.Config)
	}
}

func TestThemeLayoutHTMLUsesLayoutsDirWithoutMutatingConfig(t *testing.T) {
	root := t.TempDir()
	theme := &Theme{
		URI: &url.URL{Path: filepath.Join(root, "theme.json")},
	}
	writeTestFile(t, filepath.Join(theme.LayoutsDirPath(), "ok.html"))

	if _, err := theme.LayoutHTML("ok.html"); err != nil {
		t.Fatalf("LayoutHTML bare ref returned error: %v", err)
	}
	if _, err := theme.LayoutHTML(filepath.Join("layouts", "ok.html")); err != nil {
		t.Fatalf("LayoutHTML prefixed ref returned error: %v", err)
	}
	if theme.Config != (ThemeConfig{}) {
		t.Fatalf("LayoutHTML mutated config: %#v", theme.Config)
	}
}

func TestThemeLayoutHTMLUsesConfiguredLayoutsDir(t *testing.T) {
	root := t.TempDir()
	theme := &Theme{
		URI: &url.URL{Path: filepath.Join(root, "theme.json")},
		Config: ThemeConfig{
			LayoutsDirectory: "templates",
		},
	}
	writeTestFile(t, filepath.Join(theme.LayoutsDirPath(), "ok.html"))

	if _, err := theme.LayoutHTML("ok.html"); err != nil {
		t.Fatalf("LayoutHTML configured bare ref returned error: %v", err)
	}
	if _, err := theme.LayoutHTML(filepath.Join("templates", "ok.html")); err != nil {
		t.Fatalf("LayoutHTML configured prefixed ref returned error: %v", err)
	}
	if theme.Config.LayoutsDirectory != "templates" {
		t.Fatalf("LayoutHTML mutated layouts dir: %q", theme.Config.LayoutsDirectory)
	}
}

func TestThemeResourceHTMLRejectsUnsafeRefs(t *testing.T) {
	root := t.TempDir()
	theme := &Theme{
		URI: &url.URL{Path: filepath.Join(root, "theme.json")},
	}

	tests := []string{
		"/etc/passwd",
		"../secret.html",
		"",
		"layouts\\ok.html",
	}
	for _, ref := range tests {
		t.Run(ref, func(t *testing.T) {
			if _, err := theme.ResourceHTML(ref); err == nil {
				t.Fatalf("ResourceHTML(%q) returned nil error", ref)
			}
		})
	}
}

func TestThemeDiscoverPlugins(t *testing.T) {
	root := t.TempDir()
	theme := &Theme{
		URI: &url.URL{Path: filepath.Join(root, "theme.json")},
	}
	writeTestFileContent(t, filepath.Join(theme.PluginsDirPath(), "render.js"), "function render(input) { return input }\nconst plugin = render")
	writeTestFileContent(t, filepath.Join(theme.PluginsDirPath(), "nested", "format.js"), "function format(input) { return input }\nconst plugin = format")

	if err := theme.DiscoverPlugins(); err != nil {
		t.Fatalf("DiscoverPlugins returned error: %v", err)
	}
	if len(theme.Plugins) != 2 {
		t.Fatalf("Plugins len = %d, want 2", len(theme.Plugins))
	}
	if theme.Plugins["format"].Path != filepath.Join(theme.PluginsDirPath(), "nested", "format.js") {
		t.Fatalf("format plugin path = %q", theme.Plugins["format"].Path)
	}
	if theme.Plugins["render"].Path != filepath.Join(theme.PluginsDirPath(), "render.js") {
		t.Fatalf("render plugin path = %q", theme.Plugins["render"].Path)
	}
}

func TestListFilesByPatternInvalidRegex(t *testing.T) {
	if _, err := ListFilesByPattern(t.TempDir(), false, `(`, ""); err == nil {
		t.Fatal("ListFilesByPattern invalid include regex returned nil error")
	}
	if _, err := ListFilesByPattern(t.TempDir(), false, "", `(`); err == nil {
		t.Fatal("ListFilesByPattern invalid exclude regex returned nil error")
	}
}

func TestRelativeFilePath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "nested", "file.txt")
	writeTestFile(t, file)

	got, err := RelativeFilePath(root, file)
	if err != nil {
		t.Fatalf("RelativeFilePath returned error: %v", err)
	}
	want := "nested/file.txt"
	if got != want {
		t.Fatalf("RelativeFilePath = %q, want %q", got, want)
	}
}

func TestRelativeFilePathRejectsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "file.txt")
	writeTestFile(t, outside)

	if _, err := RelativeFilePath(root, outside); err == nil {
		t.Fatal("RelativeFilePath outside root returned nil error")
	}
}

func TestLocalFilePathRejectsUnsafeRelativePaths(t *testing.T) {
	root := t.TempDir()
	if _, err := LocalFilePath(root, "/etc/passwd"); err == nil {
		t.Fatal("LocalFilePath absolute path returned nil error")
	}
	if _, err := LocalFilePath(root, "../secrets.txt"); err == nil {
		t.Fatal("LocalFilePath parent traversal returned nil error")
	}
	if _, err := LocalFilePath(root, ""); err == nil {
		t.Fatal("LocalFilePath empty path returned nil error")
	}
}

func TestLocalFilePaths(t *testing.T) {
	root := t.TempDir()
	got, err := LocalFilePaths(root, []string{"a.txt", "nested/b.txt"})
	if err != nil {
		t.Fatalf("LocalFilePaths returned error: %v", err)
	}
	want := []string{
		filepath.Join(root, "a.txt"),
		filepath.Join(root, "nested", "b.txt"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LocalFilePaths = %#v, want %#v", got, want)
	}
}
