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

func TestLoadPlugins(t *testing.T) {
	root := t.TempDir()
	upperPath := filepath.Join(root, "upper.js")
	titlePath := filepath.Join(root, "title.js")
	if err := os.WriteFile(upperPath, []byte("// ht:plugin upper\nreturn input.toUpperCase()"), 0644); err != nil {
		t.Fatalf("write upper plugin: %v", err)
	}
	if err := os.WriteFile(titlePath, []byte("return input.replace(/\\b\\w/g, c => c.toUpperCase())"), 0644); err != nil {
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

	rendered, err := htvalue.Render(`${ page.title | upper }`, testGetter{"page": map[string]any{"title": "hello"}}, htvalue.SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render upper plugin error = %v", err)
	}
	if rendered != "HELLO" {
		t.Fatalf("Render upper plugin = %q, want HELLO", rendered)
	}

	rendered, err = htvalue.Render(`${ page.title | title }`, testGetter{"page": map[string]any{"title": "hello world"}}, htvalue.SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render title plugin error = %v", err)
	}
	if rendered != "Hello World" {
		t.Fatalf("Render title plugin = %q, want Hello World", rendered)
	}
}

func TestLoadPluginsLastPluginWins(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.js")
	secondPath := filepath.Join(root, "second.js")
	if err := os.WriteFile(firstPath, []byte("// ht:plugin label\nreturn 'first'"), 0644); err != nil {
		t.Fatalf("write first plugin: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte("// ht:plugin label\nreturn 'second'"), 0644); err != nil {
		t.Fatalf("write second plugin: %v", err)
	}

	plugins, err := LoadPlugins([]string{firstPath, secondPath})
	if err != nil {
		t.Fatalf("LoadPlugins() error = %v", err)
	}
	rendered, err := htvalue.Render(`${ page.title | label }`, testGetter{"page": map[string]any{"title": "ignored"}}, htvalue.SyntaxDefault, plugins.FuncMap())
	if err != nil {
		t.Fatalf("Render label plugin error = %v", err)
	}
	if rendered != "second" {
		t.Fatalf("Render label plugin = %q, want second", rendered)
	}
}

func TestMergeLastPluginGroupWins(t *testing.T) {
	first := Plugins{"label": {Name: "label", Path: "first.js", Func: Func(func(any) (any, error) { return "first", nil })}}
	second := Plugins{"label": {Name: "label", Path: "second.js", Func: Func(func(any) (any, error) { return "second", nil })}}

	merged := Merge(first, second)
	if merged["label"].Path != "second.js" {
		t.Fatalf("merged label path = %q, want second.js", merged["label"].Path)
	}
}

func TestPluginFuncMap(t *testing.T) {
	plugin := Plugin{
		Name: "label",
		Func: Func(func(any) (any, error) { return "ok", nil }),
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
	root := t.TempDir()
	path := filepath.Join(root, "httd.js")
	if err := os.WriteFile(path, []byte("return input"), 0644); err != nil {
		t.Fatalf("write reserved plugin: %v", err)
	}

	if _, err := LoadPlugins([]string{path}); err == nil {
		t.Fatal("LoadPlugins() reserved name error = nil")
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
