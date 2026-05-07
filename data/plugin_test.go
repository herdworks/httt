package data

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const namespacePluginSrc = `
function namespace() {
	return { count: site.pages.length, kind: typeof build }
}
`

const namespaceArrowSrc = `
const namespace = () => ({ greeting: "hello " + site.name })
`

func TestLoadPlugin_ReadsAndValidates(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "twitter.js")
	if err := os.WriteFile(path, []byte(namespacePluginSrc), 0644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}

	bindings := map[string]any{
		"site": map[string]any{
			"pages": []any{1, 2, 3, 4},
		},
		"build": "ssg",
	}
	plugin, err := LoadPlugin(path, bindings)
	if err != nil {
		t.Fatalf("LoadPlugin error = %v", err)
	}
	if plugin.Name != "twitter" {
		t.Fatalf("Name = %q, want twitter", plugin.Name)
	}
	if plugin.Path != path {
		t.Fatalf("Path = %q, want %q", plugin.Path, path)
	}

	got, err := plugin.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Run = %T, want map[string]any", got)
	}
	if count, _ := m["count"].(int64); count != 4 {
		t.Fatalf("count = %v, want 4", m["count"])
	}
	if kind, _ := m["kind"].(string); kind != "string" {
		t.Fatalf("kind = %v, want string", m["kind"])
	}
}

func TestLoadPlugin_ArrowEntrypoint(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "greeting.js")
	if err := os.WriteFile(path, []byte(namespaceArrowSrc), 0644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	bindings := map[string]any{"site": map[string]any{"name": "world"}}
	plugin, err := LoadPlugin(path, bindings)
	if err != nil {
		t.Fatalf("LoadPlugin error = %v", err)
	}
	got, err := plugin.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}
	m, _ := got.(map[string]any)
	if greeting, _ := m["greeting"].(string); greeting != "hello world" {
		t.Fatalf("greeting = %q, want %q", greeting, "hello world")
	}
}

func TestLoadPlugin_RejectsReservedNames(t *testing.T) {
	for _, name := range []string{"namespace", "data"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name+".js")
			if err := os.WriteFile(path, []byte(namespacePluginSrc), 0644); err != nil {
				t.Fatalf("write plugin: %v", err)
			}
			if _, err := LoadPlugin(path, nil); err == nil {
				t.Fatalf("LoadPlugin(%s.js) reserved-name error = nil", name)
			} else if !strings.Contains(err.Error(), "reserved namespace name") {
				t.Fatalf("LoadPlugin(%s.js) error = %v, want substring %q", name, err, "reserved namespace name")
			}
		})
	}
}

func TestLoadPlugin_RejectsInvalidNames(t *testing.T) {
	for _, name := range []string{
		"bad-name",  // hyphen
		"has space", // space
		"foo.bar",   // intermediate dot — produces "foo.bar" before extension trim, fails regex
		"$dollar",   // dollar sign
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name+".js")
			if err := os.WriteFile(path, []byte(namespacePluginSrc), 0644); err != nil {
				t.Fatalf("write plugin: %v", err)
			}
			if _, err := LoadPlugin(path, nil); err == nil {
				t.Fatalf("LoadPlugin(%q) invalid-name error = nil", name)
			} else if !strings.Contains(err.Error(), "invalid namespace name") {
				t.Fatalf("LoadPlugin(%q) error = %v, want substring %q", name, err, "invalid namespace name")
			}
		})
	}
}

func TestLoadPlugin_AllowsLeadingDigit(t *testing.T) {
	// Namespace names accept leading digits; that's the deliberate
	// difference from value-pipeline plugin names. Keypath segments
	// happily resolve numeric-looking keys as map lookups.
	root := t.TempDir()
	path := filepath.Join(root, "2024.js")
	if err := os.WriteFile(path, []byte(namespacePluginSrc), 0644); err != nil {
		t.Fatalf("write plugin: %v", err)
	}
	bindings := map[string]any{"site": map[string]any{"pages": []any{1}}, "build": "x"}
	plugin, err := LoadPlugin(path, bindings)
	if err != nil {
		t.Fatalf("LoadPlugin error = %v", err)
	}
	if plugin.Name != "2024" {
		t.Fatalf("Name = %q, want 2024", plugin.Name)
	}
}

func TestNamespaceSegmentsFromRelPath(t *testing.T) {
	tests := []struct {
		rel  string
		want []string
	}{
		{"twitter.js", []string{"twitter"}},
		{"social/twitter.js", []string{"social", "twitter"}},
		{"foo/bar/baz.js", []string{"foo", "bar", "baz"}},
		{"foo.yaml", []string{"foo"}},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			got, err := NamespaceSegmentsFromRelPath(tt.rel)
			if err != nil {
				t.Fatalf("NamespaceSegmentsFromRelPath(%q) error = %v", tt.rel, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("segments = %v, want %v", got, tt.want)
			}
			for i, seg := range got {
				if seg != tt.want[i] {
					t.Fatalf("segments[%d] = %q, want %q", i, seg, tt.want[i])
				}
			}
		})
	}
}

func TestNamespaceSegmentsFromRelPath_Invalid(t *testing.T) {
	for _, rel := range []string{
		"",
		"   ",
		"bad name.js",
		"good/bad-name.js",
	} {
		t.Run(rel, func(t *testing.T) {
			if _, err := NamespaceSegmentsFromRelPath(rel); err == nil {
				t.Fatalf("NamespaceSegmentsFromRelPath(%q) error = nil", rel)
			}
		})
	}
}
