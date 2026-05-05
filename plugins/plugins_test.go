package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	htvalue "github.com/herdworks/hypertemplates-go/value"
)

type testGetter map[string]any

func (getter testGetter) Get(keypath string) any {
	value := any(map[string]any(getter))

	for _, key := range strings.Split(keypath, ".") {
		mapped, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = mapped[key]
	}
	return value
}

const upperPluginSrc = `
function plugin(input) {
	return input.toUpperCase()
}
`

const titlePluginSrc = `
const plugin = (input) => input.replace(/\b\w/g, c => c.toUpperCase())
`

func TestLoadPlugins(t *testing.T) {
	root := t.TempDir()
	upperPath := filepath.Join(root, "upper.js")
	titlePath := filepath.Join(root, "title.js")
	if err := os.WriteFile(upperPath, []byte(upperPluginSrc), 0644); err != nil {
		t.Fatalf("write upper plugin: %v", err)
	}
	if err := os.WriteFile(titlePath, []byte(titlePluginSrc), 0644); err != nil {
		t.Fatalf("write title plugin: %v", err)
	}

	plugins, err := LoadPlugins([]string{upperPath, titlePath})
	if err != nil {
		t.Fatalf("LoadPlugins() error = %v", err)
	}
	if plugins["upper"].Name != "upper" {
		t.Fatalf("upper plugin name = %q, want upper", plugins["upper"].Name)
	}
	if plugins["upper"].Path != upperPath {
		t.Fatalf("upper plugin path = %q, want %q", plugins["upper"].Path, upperPath)
	}

	rendered, err := htvalue.RenderString(`${ page.title | upper }`, testGetter{"page": map[string]any{"title": "hello"}}, htvalue.SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render upper plugin error = %v", err)
	}
	if rendered != "HELLO" {
		t.Fatalf("Render upper plugin = %q, want HELLO", rendered)
	}

	rendered, err = htvalue.RenderString(`${ page.title | title }`, testGetter{"page": map[string]any{"title": "hello world"}}, htvalue.SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render title plugin error = %v", err)
	}
	if rendered != "Hello World" {
		t.Fatalf("Render title plugin = %q, want Hello World", rendered)
	}
}

func TestLoadPluginsLastPluginWins(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "label.js")
	secondPath := filepath.Join(root, "label.js")
	// Two distinct files in two distinct dirs, both named label.js.
	firstDir := filepath.Join(root, "first")
	secondDir := filepath.Join(root, "second")
	if err := os.MkdirAll(firstDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(secondDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	firstPath = filepath.Join(firstDir, "label.js")
	secondPath = filepath.Join(secondDir, "label.js")
	if err := os.WriteFile(firstPath, []byte("function plugin() { return 'first' }"), 0644); err != nil {
		t.Fatalf("write first plugin: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("function plugin() { return 'second' }"), 0644); err != nil {
		t.Fatalf("write second plugin: %v", err)
	}

	plugins, err := LoadPlugins([]string{firstPath, secondPath})
	if err != nil {
		t.Fatalf("LoadPlugins() error = %v", err)
	}
	rendered, err := htvalue.RenderString(`${ page.title | label }`, testGetter{"page": map[string]any{"title": "ignored"}}, htvalue.SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render label plugin error = %v", err)
	}
	if rendered != "second" {
		t.Fatalf("Render label plugin = %q, want second", rendered)
	}
}

func TestMergeLastPluginGroupWins(t *testing.T) {
	first := Plugins{"label": {Name: "label", Path: "first.js", Func: Func(func(...any) (any, error) { return "first", nil })}}
	second := Plugins{"label": {Name: "label", Path: "second.js", Func: Func(func(...any) (any, error) { return "second", nil })}}

	merged := Merge(first, second)
	if merged["label"].Path != "second.js" {
		t.Fatalf("merged label path = %q, want second.js", merged["label"].Path)
	}
}

func TestPluginFuncMap(t *testing.T) {
	plugin := Plugin{
		Name: "label",
		Func: Func(func(...any) (any, error) { return "ok", nil }),
	}

	funcs := plugin.FuncMap()
	if len(funcs) != 1 {
		t.Fatalf("FuncMap len = %d, want 1", len(funcs))
	}
	if funcs["label"] == nil {
		t.Fatal("FuncMap label function = nil")
	}
}

func TestLoadPluginsRejectsReservedNames(t *testing.T) {
	for _, name := range []string{"get", "plugin"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name+".js")
			if err := os.WriteFile(path, []byte("function plugin(input) { return input }"), 0644); err != nil {
				t.Fatalf("write reserved plugin: %v", err)
			}

			if _, err := LoadPlugins([]string{path}); err == nil {
				t.Fatalf("LoadPlugins(%s.js) reserved-name error = nil", name)
			} else if !strings.Contains(err.Error(), "reserved plugin name") {
				t.Fatalf("LoadPlugins(%s.js) error = %v, want substring %q", name, err, "reserved plugin name")
			}
		})
	}
}

func TestLoadPluginsRejectsInvalidFilenames(t *testing.T) {
	for _, name := range []string{
		"bad-name",  // hyphen
		"2digits",   // leading digit
		"has space", // space
		"foo.bar",   // intermediate dot
		"emoji-🚀",   // non-ASCII (also has hyphen)
		"-leading",  // leading hyphen
		"$dollar",   // dollar sign
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name+".js")
			if err := os.WriteFile(path, []byte("function plugin(input) { return input }"), 0644); err != nil {
				t.Fatalf("write invalid plugin: %v", err)
			}

			if _, err := LoadPlugins([]string{path}); err == nil {
				t.Fatalf("LoadPlugins(%q) invalid-name error = nil", name)
			} else if !strings.Contains(err.Error(), "invalid plugin name") {
				t.Fatalf("LoadPlugins(%q) error = %v, want substring %q", name, err, "invalid plugin name")
			}
		})
	}
}

func TestLoadPluginsReportsCompileErrors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "broken.js")
	if err := os.WriteFile(path, []byte("return }"), 0644); err != nil {
		t.Fatalf("write broken plugin: %v", err)
	}

	if _, err := LoadPlugins([]string{path}); err == nil {
		t.Fatal("LoadPlugins() compile error = nil")
	}
}

func TestLoadPluginsRequiresPluginFunction(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "upper.js")
	// No `function plugin` and no `const plugin = ...`.
	if err := os.WriteFile(path, []byte("function upper(input) { return input.toUpperCase() }"), 0644); err != nil {
		t.Fatalf("write missing-entry plugin: %v", err)
	}

	plugins, err := LoadPlugins([]string{path})
	if err != nil {
		t.Fatalf("LoadPlugins() unexpected compile error: %v", err)
	}
	// Compile succeeds (LoadPlugin doesn't run the script). The error is
	// surfaced when the plugin is invoked, since the wrapper checks
	// typeof plugin === "function".
	if _, err := plugins["upper"].Func("hello"); err == nil {
		t.Fatal("plugins[upper].Func() missing-entry error = nil")
	}
}

func TestLoadPluginsVariadicPipelineArgs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fmtdate.js")
	if err := os.WriteFile(path, []byte(`
function plugin(format = "", input = "") {
	if (typeof input !== "string") {
		return input
	}
	return format + ":" + input
}
`), 0644); err != nil {
		t.Fatalf("write fmtdate plugin: %v", err)
	}

	plugins, err := LoadPlugins([]string{path})
	if err != nil {
		t.Fatalf("LoadPlugins() error = %v", err)
	}

	rendered, err := htvalue.RenderString(
		`${ page.updated_at, page.created_at | fmtdate "YYYY-MM-DD" }`,
		testGetter{"page": map[string]any{"created_at": "2026-04-28T00:00:00Z"}},
		htvalue.SyntaxDefault,
		plugins.FuncMap(),
	)
	if err != nil {
		t.Fatalf("Render fmtdate plugin error = %v", err)
	}
	if rendered != "YYYY-MM-DD:2026-04-28T00:00:00Z" {
		t.Fatalf("Render fmtdate plugin = %q, want formatted fallback date", rendered)
	}
}
