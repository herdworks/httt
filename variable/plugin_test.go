package variable

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const upperPluginSrc = `
export default function(input) {
	return input.toUpperCase()
}
`

const titlePluginSrc = `
export default (input) => input.replace(/\b\w/g, c => c.toUpperCase())
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

	rendered, err := RenderString(`${ upper(page.title) }`, testGetter{"page": map[string]any{"title": "hello"}}, SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render upper plugin error = %v", err)
	}
	if rendered != "HELLO" {
		t.Fatalf("Render upper plugin = %q, want HELLO", rendered)
	}

	rendered, err = RenderString(`${ title(page.title) }`, testGetter{"page": map[string]any{"title": "hello world"}}, SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render title plugin error = %v", err)
	}
	if rendered != "Hello World" {
		t.Fatalf("Render title plugin = %q, want Hello World", rendered)
	}
}

func TestLoadPluginsLastPluginWins(t *testing.T) {
	root := t.TempDir()
	firstDir := filepath.Join(root, "first")
	secondDir := filepath.Join(root, "second")
	if err := os.MkdirAll(firstDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(secondDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	firstPath := filepath.Join(firstDir, "label.js")
	secondPath := filepath.Join(secondDir, "label.js")
	if err := os.WriteFile(firstPath, []byte("export default function() { return 'first' }"), 0644); err != nil {
		t.Fatalf("write first plugin: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("export default function() { return 'second' }"), 0644); err != nil {
		t.Fatalf("write second plugin: %v", err)
	}

	plugins, err := LoadPlugins([]string{firstPath, secondPath})
	if err != nil {
		t.Fatalf("LoadPlugins() error = %v", err)
	}
	rendered, err := RenderString(`${ label(page.title) }`, testGetter{"page": map[string]any{"title": "ignored"}}, SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render label plugin error = %v", err)
	}
	if rendered != "second" {
		t.Fatalf("Render label plugin = %q, want second", rendered)
	}
}

func TestMergePluginsLastPluginGroupWins(t *testing.T) {
	first := Plugins{"label": {Name: "label", Path: "first.js", Func: func(...any) (any, error) { return "first", nil }}}
	second := Plugins{"label": {Name: "label", Path: "second.js", Func: func(...any) (any, error) { return "second", nil }}}

	merged := MergePlugins(first, second)
	if merged["label"].Path != "second.js" {
		t.Fatalf("merged label path = %q, want second.js", merged["label"].Path)
	}
}

func TestPluginFuncMap(t *testing.T) {
	plugin := Plugin{
		Name: "label",
		Func: func(...any) (any, error) { return "ok", nil },
	}

	funcs := plugin.FuncMap()
	if len(funcs) != 1 {
		t.Fatalf("FuncMap len = %d, want 1", len(funcs))
	}
	if funcs["label"] == nil {
		t.Fatal("FuncMap label function = nil")
	}
}

func TestPluginsFuncMap_SkipsNilFunc(t *testing.T) {
	plugins := Plugins{
		"good": {Name: "good", Func: func(...any) (any, error) { return "ok", nil }},
		"bad":  {Name: "bad"},
	}
	funcs := plugins.FuncMap()
	if _, ok := funcs["good"]; !ok {
		t.Fatal("good plugin missing from FuncMap")
	}
	if _, ok := funcs["bad"]; ok {
		t.Fatal("bad plugin (nil Func) leaked into FuncMap")
	}
}

func TestLoadPluginsRejectsReservedNames(t *testing.T) {
	for _, name := range []string{"get", "text", "markdown"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name+".js")
			if err := os.WriteFile(path, []byte("export default function(input) { return input }"), 0644); err != nil {
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
			if err := os.WriteFile(path, []byte("export default function(input) { return input }"), 0644); err != nil {
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

func TestLoadPluginsRequiresDefaultExport(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "upper.js")
	// No default export — just a free function declaration.
	if err := os.WriteFile(path, []byte("function upper(input) { return input.toUpperCase() }"), 0644); err != nil {
		t.Fatalf("write missing-entry plugin: %v", err)
	}

	plugins, err := LoadPlugins([]string{path})
	if err != nil {
		t.Fatalf("LoadPlugins() unexpected compile error: %v", err)
	}
	// Compile succeeds (LoadPlugin doesn't run the script). The error is
	// surfaced when the plugin is invoked: the wrapper extracts
	// module.exports.default (or module.exports) and asserts callability.
	if _, err := plugins["upper"].Func("hello"); err == nil {
		t.Fatal("plugins[upper].Func() missing-default error = nil")
	}
}

func TestLoadPluginsFunctionArgumentsInSourceOrder(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fmtdate.js")
	if err := os.WriteFile(path, []byte(`
export default function(updated = null, created = null, format = "") {
	const input = updated ?? created
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

	rendered, err := RenderString(
		`${ fmtdate(page.updated_at, page.created_at, "YYYY-MM-DD") }`,
		testGetter{"page": map[string]any{"created_at": "2026-04-28T00:00:00Z"}},
		SyntaxDefault,
		plugins.FuncMap(),
	)
	if err != nil {
		t.Fatalf("Render fmtdate plugin error = %v", err)
	}
	if rendered != "YYYY-MM-DD:2026-04-28T00:00:00Z" {
		t.Fatalf("Render fmtdate plugin = %q, want formatted fallback date", rendered)
	}
}
