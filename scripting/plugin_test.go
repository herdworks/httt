package scripting

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const upperPluginSrc = `
function plugin(input) {
	return input.toUpperCase()
}
`

const arrowPluginSrc = `
const plugin = (input) => input.toUpperCase()
`

const namespacePluginSrc = `
function namespace() {
	return { count: site.pages.length }
}
`

func TestLoadPlugin_DerivesNameAndKeepsSource(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "upper.js")
	if err := os.WriteFile(path, []byte(upperPluginSrc), 0644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	plugin, err := LoadPlugin(path)
	if err != nil {
		t.Fatalf("LoadPlugin error = %v", err)
	}
	if plugin.Name != "upper" {
		t.Fatalf("Name = %q, want upper", plugin.Name)
	}
	if plugin.Path != path {
		t.Fatalf("Path = %q, want %q", plugin.Path, path)
	}
	if !strings.Contains(plugin.Source, "function plugin") {
		t.Fatalf("Source missing entrypoint declaration: %q", plugin.Source)
	}
	if plugin.Entrypoint != "" {
		t.Fatalf("Entrypoint = %q, want empty (caller sets it)", plugin.Entrypoint)
	}
	if plugin.Bindings != nil {
		t.Fatalf("Bindings = %v, want nil", plugin.Bindings)
	}
}

func TestLoadPlugins_PreservesOrder(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "a.js"),
		filepath.Join(root, "b.js"),
		filepath.Join(root, "c.js"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("function plugin() { return 0 }"), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	plugins, err := LoadPlugins(paths)
	if err != nil {
		t.Fatalf("LoadPlugins error = %v", err)
	}
	if len(plugins) != 3 {
		t.Fatalf("len(plugins) = %d, want 3", len(plugins))
	}
	for i, want := range []string{"a", "b", "c"} {
		if plugins[i].Name != want {
			t.Fatalf("plugins[%d].Name = %q, want %q", i, plugins[i].Name, want)
		}
	}
}

func TestLoadPlugin_ReadsErrorsAreReported(t *testing.T) {
	_, err := LoadPlugin(filepath.Join(t.TempDir(), "missing.js"))
	if err == nil {
		t.Fatal("LoadPlugin missing-file error = nil")
	}
	if !strings.Contains(err.Error(), "read plugin") {
		t.Fatalf("LoadPlugin error = %v, want substring %q", err, "read plugin")
	}
}

func TestPluginCompile_RoundTrip(t *testing.T) {
	plugin := Plugin{
		Name:       "upper",
		Source:     upperPluginSrc,
		Entrypoint: "plugin",
	}
	fn, err := plugin.Compile()
	if err != nil {
		t.Fatalf("Compile error = %v", err)
	}
	got, err := fn("hello")
	if err != nil {
		t.Fatalf("Call error = %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("Call = %q, want HELLO", got)
	}
}

func TestPluginCompile_ArrowEntrypoint(t *testing.T) {
	plugin := Plugin{
		Name:       "upper",
		Source:     arrowPluginSrc,
		Entrypoint: "plugin",
	}
	fn, err := plugin.Compile()
	if err != nil {
		t.Fatalf("Compile error = %v", err)
	}
	got, err := fn("hello")
	if err != nil {
		t.Fatalf("Call error = %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("Call = %q, want HELLO", got)
	}
}

func TestPluginCompile_RejectsBindings(t *testing.T) {
	plugin := Plugin{
		Name:       "upper",
		Source:     upperPluginSrc,
		Entrypoint: "plugin",
		Bindings:   map[string]any{"site": map[string]any{}},
	}
	if _, err := plugin.Compile(); err == nil {
		t.Fatal("Compile with Bindings: error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "use Run") {
		t.Fatalf("Compile error = %v, want substring %q", err, "use Run")
	}
}

func TestPluginCompile_RequiresEntrypoint(t *testing.T) {
	plugin := Plugin{Name: "upper", Source: upperPluginSrc}
	if _, err := plugin.Compile(); err == nil {
		t.Fatal("Compile without Entrypoint: error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "Entrypoint is required") {
		t.Fatalf("Compile error = %v, want substring %q", err, "Entrypoint is required")
	}
}

func TestPluginRun_WithBindings(t *testing.T) {
	plugin := Plugin{
		Name:       "stats",
		Source:     namespacePluginSrc,
		Entrypoint: "namespace",
		Bindings: map[string]any{
			"site": map[string]any{
				"pages": []any{1, 2, 3},
			},
		},
	}
	got, err := plugin.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Run = %T, want map[string]any", got)
	}
	if count, _ := m["count"].(int64); count != 3 {
		t.Fatalf("count = %v, want 3", m["count"])
	}
}

func TestPluginRun_RequiresEntrypoint(t *testing.T) {
	plugin := Plugin{Source: namespacePluginSrc}
	if _, err := plugin.Run(context.Background()); err == nil {
		t.Fatal("Run without Entrypoint: error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "Entrypoint is required") {
		t.Fatalf("Run error = %v", err)
	}
}

func TestPluginRunTimeout_RejectsZero(t *testing.T) {
	plugin := Plugin{
		Name:       "stats",
		Source:     namespacePluginSrc,
		Entrypoint: "namespace",
	}
	if _, err := plugin.RunTimeout(0); err == nil {
		t.Fatal("RunTimeout(0) error = nil, want non-nil")
	}
}
