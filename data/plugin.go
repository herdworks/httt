package data

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/herdworks/hypertemplates-go/scripting"
)

// PluginEntrypoint is the JavaScript entrypoint name expected in
// computed-namespace plugin files. Plugin sources must declare a top-level
// `function namespace()` or `const namespace = <callable>`.
const PluginEntrypoint = "namespace"

// Plugin is a computed-namespace plugin: a JavaScript file whose
// `namespace` entrypoint is invoked once with caller-provided bindings.
// The return value is installed at the file's keypath in the surrounding
// TemplateData. Bindings expose a snapshot of the build-time environment
// (typically `ht`, `site`, `build`, `env`, `theme`, `data`) as lexical
// locals to the running JavaScript.
type Plugin struct {
	Name     string
	Path     string
	Bindings map[string]any
	raw      scripting.Plugin
}

// Run evaluates the plugin's `namespace` entrypoint with no arguments and
// returns its result. Bindings declared on the Plugin are exposed as
// lexical locals during evaluation.
func (p Plugin) Run(ctx context.Context) (any, error) {
	plugin := p.raw
	plugin.Entrypoint = PluginEntrypoint
	plugin.Bindings = p.Bindings
	return plugin.Run(ctx)
}

// LoadPlugin reads a JavaScript file from path, validates that the
// derived plugin name conforms to the namespace naming rules, and returns
// a Plugin ready to Run. bindings is captured by reference and exposed to
// the JS runtime when Run is called.
func LoadPlugin(path string, bindings map[string]any) (Plugin, error) {
	raw, err := scripting.LoadPlugin(path)
	if err != nil {
		return Plugin{}, err
	}
	if err := validateNamespaceName(raw.Name); err != nil {
		return Plugin{}, fmt.Errorf("data: load %q: %w", path, err)
	}
	return Plugin{
		Name:     raw.Name,
		Path:     raw.Path,
		Bindings: bindings,
		raw:      raw,
	}, nil
}

// namespaceSegmentRegexp constrains a single namespace path segment to
// letters, digits, and underscores. Segments are looked up as TemplateData
// map keys (not Go identifiers); the regex rejects whitespace, dots,
// hyphens, and any other character that would interact awkwardly with the
// keypath syntax.
var namespaceSegmentRegexp = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// reservedNamespaceNames are leaf-segment names reserved by the
// computed-namespace plugin convention:
//   - "namespace": the JS entrypoint name expected in plugin files.
//   - "data": reserved to avoid colliding with the top-level data namespace.
var reservedNamespaceNames = map[string]struct{}{
	PluginEntrypoint: {},
	"data":           {},
}

// NamespaceSegmentsFromRelPath turns a slash-separated relative path like
// "social/twitter.js" into the keypath segments ["social", "twitter"].
// Returns an error if any segment fails namespaceSegmentRegexp.
func NamespaceSegmentsFromRelPath(rel string) ([]string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return nil, fmt.Errorf("empty path")
	}
	parts := strings.Split(rel, "/")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty path")
	}
	last := parts[len(parts)-1]
	if ext := filepath.Ext(last); ext != "" {
		parts[len(parts)-1] = strings.TrimSuffix(last, ext)
	}
	for _, seg := range parts {
		if !namespaceSegmentRegexp.MatchString(seg) {
			return nil, fmt.Errorf("invalid namespace segment %q (must match %s)", seg, namespaceSegmentRegexp.String())
		}
	}
	return parts, nil
}

func validateNamespaceName(name string) error {
	if !namespaceSegmentRegexp.MatchString(name) {
		return fmt.Errorf("invalid namespace name %q (must match %s)", name, namespaceSegmentRegexp.String())
	}
	if _, reserved := reservedNamespaceNames[name]; reserved {
		return fmt.Errorf("reserved namespace name %q", name)
	}
	return nil
}
