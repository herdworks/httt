package scripting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Plugin is a JavaScript file whose default export is a function. Plugin
// authors write idiomatic ES module syntax:
//
//	export default function(input) {
//		return input.toUpperCase()
//	}
//
// and the engine takes care of:
//
//   - Pre-scanning for unsupported features (`import`, `export ... from`)
//     and surfacing them as load-time *PluginError values.
//   - Transforming ESM to a CommonJS-shaped form via esbuild.
//   - Wrapping the result in an IIFE that synthesizes module/exports and a
//     require() stub (which throws a runtime PluginError if invoked), then
//     extracts the default-exported callable.
//
// Plugin distinguishes two host-side properties:
//
//   - bindings: optional top-level local consts injected before the user's
//     source executes (used by computed-data plugins that need access to
//     build-time data like `data`, `userData`, `site`). The plugin's body
//     references these as plain identifiers.
//   - per-call arguments: passed positionally to the default-exported
//     function on every invocation (used by value-pipeline plugins).
//
// Both flow through Compile uniformly.
type Plugin struct {
	// Name is a logical identifier for the plugin, typically derived from
	// the source file name (e.g. "upper" for upper.js). Consumer packages
	// are responsible for naming-rule validation and collision handling.
	Name string

	// Path is the source file path, retained for diagnostics. Optional.
	Path string

	// Source is the JavaScript source as authored (ESM, with `export
	// default <function>`).
	Source string
}

// PluginCallable is the shape returned by Plugin.Compile. ctx supervises
// the JS execution; bindings supplies values for the top-level lexical
// locals declared by Compile's bindingsKeys argument (pass nil if the
// plugin has no bindings); args are forwarded positionally to the
// default-exported function.
type PluginCallable func(ctx context.Context, bindings map[string]any, args ...any) (any, error)

// Compile transforms the plugin source, builds and caches the wrapped
// program, and returns a callable. bindingsKeys names the top-level lexical
// locals the wrapper exposes from the bindings argument; pass nil for
// plugins that take no bindings. The returned callable is safe for
// concurrent use.
//
// Errors returned at Compile time are typically *PluginError (unsupported
// syntax detected by the pre-scan) or wrapped esbuild/goja compile errors.
// Errors returned by the callable can be *PluginError when JavaScript
// throws a PluginError-tagged exception (e.g. invoking the require stub).
func (p Plugin) Compile(bindingsKeys []string) (PluginCallable, error) {
	keys := append([]string(nil), bindingsKeys...)
	sort.Strings(keys)

	for _, k := range keys {
		if !isValidJSIdentifier(k) {
			return nil, fmt.Errorf("scripting: Plugin %q: bindings key %q is not a valid JS identifier", p.Name, k)
		}
	}

	transformed, err := transform(p.Source, p.Path)
	if err != nil {
		return nil, err
	}

	prog, err := shared().programForPlugin(transformed, keys)
	if err != nil {
		return nil, fmt.Errorf("scripting: Plugin %q: %w", p.Name, err)
	}

	path := p.Path

	return func(ctx context.Context, bindings map[string]any, args ...any) (any, error) {
		v, err := shared().evalPluginProgram(ctx, prog, bindings, args)
		if err != nil {
			if perr := asPluginError(err, path); perr != nil {
				return nil, perr
			}
			return nil, err
		}
		if v == nil {
			return nil, nil
		}
		return v.Export(), nil
	}, nil
}

// LoadPlugin reads a JavaScript file from path and returns a Plugin with
// Name derived from the file's basename (sans extension), Path set to path,
// and Source set to the file contents.
func LoadPlugin(path string) (Plugin, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Plugin{}, fmt.Errorf("scripting: read plugin %q: %w", path, err)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return Plugin{
		Name:   name,
		Path:   path,
		Source: string(src),
	}, nil
}

// LoadPlugins reads multiple JavaScript files, in input order. The returned
// slice preserves order so callers can implement "last wins" semantics by
// index.
func LoadPlugins(paths []string) ([]Plugin, error) {
	plugins := make([]Plugin, 0, len(paths))
	for _, path := range paths {
		plugin, err := LoadPlugin(path)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}
