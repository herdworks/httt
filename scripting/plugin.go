package scripting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Plugin is a JavaScript file with a named entrypoint and optional local
// bindings.
//
// Two evaluation modes are supported:
//
//   - Compile() produces a reusable callable that invokes Entrypoint with
//     caller-provided arguments. Use this for plugins where the same
//     function is invoked many times during a build (e.g. value-pipeline
//     plugins).
//
//   - Run(ctx) evaluates the plugin once. If Bindings is non-nil, each
//     binding is exposed as a lexical local in JavaScript. Entrypoint is
//     called with no arguments. Use this for plugins where the JS runs
//     once at build time to produce a value (e.g. computed-data plugins).
//
// A plugin with Bindings should use Run; a plugin that needs per-call
// arguments should use Compile.
type Plugin struct {
	// Name is a logical identifier for the plugin, typically derived from
	// the source file name (e.g. "upper" for upper.js). Consumer packages
	// are responsible for naming-rule validation and collision handling.
	Name string

	// Path is the source file path, retained for diagnostics. Optional.
	Path string

	// Source is the JavaScript source.
	Source string

	// Entrypoint is the name of the top-level JS function or const-bound
	// callable that runtime invocation targets (e.g. "plugin",
	// "namespace"). Required.
	Entrypoint string

	// Bindings, when non-nil, exposes each map key as a lexical local in
	// the JavaScript runtime during Run. Bindings are scoped to the call
	// and do not leak across pooled runtimes. Compile rejects plugins with
	// Bindings; for that mode use Run.
	Bindings map[string]any
}

// Compile returns a reusable callable that invokes Entrypoint with
// caller-provided arguments. Each call uses the package-default timeout.
func (p Plugin) Compile() (func(...any) (any, error), error) {
	if err := p.validateForCompile(); err != nil {
		return nil, err
	}
	return shared().NamedFunctionTimeout(p.Source, p.Entrypoint, defaultFunctionTimeout)
}

// CompileContext returns a reusable callable that accepts a caller-owned
// context on every invocation.
func (p Plugin) CompileContext() (func(context.Context, ...any) (any, error), error) {
	if err := p.validateForCompile(); err != nil {
		return nil, err
	}
	return shared().NamedFunctionContext(p.Source, p.Entrypoint)
}

// CompileTimeout returns a reusable callable that applies timeout to every
// invocation.
func (p Plugin) CompileTimeout(timeout time.Duration) (func(...any) (any, error), error) {
	if err := p.validateForCompile(); err != nil {
		return nil, err
	}
	return shared().NamedFunctionTimeout(p.Source, p.Entrypoint, timeout)
}

// Run evaluates the plugin once and returns Entrypoint's return value.
// If Bindings is non-nil, each binding is exposed as a lexical local.
// Entrypoint is called with no arguments.
func (p Plugin) Run(ctx context.Context) (any, error) {
	if p.Entrypoint == "" {
		return nil, fmt.Errorf("scripting: Plugin %q: Entrypoint is required", p.Name)
	}
	return shared().SeededFunction(ctx, p.Source, p.Entrypoint, p.Bindings)
}

// RunTimeout is the timeout variant of Run.
func (p Plugin) RunTimeout(timeout time.Duration) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.Run(ctx)
}

func (p Plugin) validateForCompile() error {
	if p.Entrypoint == "" {
		return fmt.Errorf("scripting: Plugin %q: Entrypoint is required", p.Name)
	}
	if len(p.Bindings) > 0 {
		return fmt.Errorf("scripting: Plugin %q has bindings; use Run, not Compile", p.Name)
	}
	return nil
}

// LoadPlugin reads a JavaScript file from path and returns a Plugin with
// Name derived from the file's basename (sans extension), Path set to path,
// and Source set to the file contents. Entrypoint and Bindings are not set;
// callers are responsible for setting them and for validating Name against
// any kind-specific naming rules.
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
// index. Entrypoint and Bindings are not set on the returned plugins.
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
