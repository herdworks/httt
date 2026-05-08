package scripting

import (
	"context"
	"errors"
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

const arrowPluginSrc = `
export default (input) => input.toUpperCase()
`

const namespacePluginSrc = `
export default function() {
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
	if !strings.Contains(plugin.Source, "export default") {
		t.Fatalf("Source missing default export: %q", plugin.Source)
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
		if err := os.WriteFile(path, []byte("export default function() { return 0 }"), 0644); err != nil {
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
	plugin := Plugin{Name: "upper", Source: upperPluginSrc}
	fn, err := plugin.Compile(nil)
	if err != nil {
		t.Fatalf("Compile error = %v", err)
	}
	got, err := fn(context.Background(), nil, "hello")
	if err != nil {
		t.Fatalf("Call error = %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("Call = %q, want HELLO", got)
	}
}

func TestPluginCompile_ArrowDefaultExport(t *testing.T) {
	plugin := Plugin{Name: "upper", Source: arrowPluginSrc}
	fn, err := plugin.Compile(nil)
	if err != nil {
		t.Fatalf("Compile error = %v", err)
	}
	got, err := fn(context.Background(), nil, "hello")
	if err != nil {
		t.Fatalf("Call error = %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("Call = %q, want HELLO", got)
	}
}

func TestPluginCompile_WithBindings(t *testing.T) {
	plugin := Plugin{Name: "stats", Source: namespacePluginSrc}
	fn, err := plugin.Compile([]string{"site"})
	if err != nil {
		t.Fatalf("Compile error = %v", err)
	}
	bindings := map[string]any{
		"site": map[string]any{
			"pages": []any{1, 2, 3},
		},
	}
	got, err := fn(context.Background(), bindings)
	if err != nil {
		t.Fatalf("Call error = %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Call = %T, want map[string]any", got)
	}
	if count, _ := m["count"].(int64); count != 3 {
		t.Fatalf("count = %v, want 3", m["count"])
	}
}

func TestPluginCompile_RejectsInvalidBindingsKey(t *testing.T) {
	plugin := Plugin{Name: "stats", Source: namespacePluginSrc}
	if _, err := plugin.Compile([]string{"bad-name"}); err == nil {
		t.Fatal("Compile with invalid bindings key: error = nil, want non-nil")
	}
}

func TestPluginCompile_RejectsImport(t *testing.T) {
	src := `
import foo from './foo.js'
export default function() { return foo }
`
	plugin := Plugin{Name: "x", Source: src, Path: "x.js"}
	_, err := plugin.Compile(nil)
	if err == nil {
		t.Fatal("Compile with import: error = nil, want *PluginError")
	}
	var perr *PluginError
	if !errors.As(err, &perr) {
		t.Fatalf("Compile error = %v, want *PluginError", err)
	}
	if perr.Feature != "import" {
		t.Fatalf("PluginError.Feature = %q, want import", perr.Feature)
	}
}

func TestPluginCompile_RuntimeRequireSurfacesPluginError(t *testing.T) {
	src := `
const lib = require('lodash')
export default function() { return lib }
`
	plugin := Plugin{Name: "x", Source: src, Path: "x.js"}
	fn, err := plugin.Compile(nil)
	if err != nil {
		t.Fatalf("Compile error = %v", err)
	}
	_, err = fn(context.Background(), nil)
	if err == nil {
		t.Fatal("Call invoking require: error = nil, want *PluginError")
	}
	var perr *PluginError
	if !errors.As(err, &perr) {
		t.Fatalf("Call error = %v, want *PluginError", err)
	}
	if perr.Feature != "require" {
		t.Fatalf("PluginError.Feature = %q, want require", perr.Feature)
	}
}

func TestPluginCompile_NonFunctionDefault(t *testing.T) {
	src := `export default 42`
	plugin := Plugin{Name: "x", Source: src}
	fn, err := plugin.Compile(nil)
	if err != nil {
		t.Fatalf("Compile error = %v", err)
	}
	if _, err := fn(context.Background(), nil); err == nil {
		t.Fatal("Call with non-function default: error = nil, want non-nil")
	}
}
