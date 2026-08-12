package variable

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"time"

	"github.com/herdworks/httt/scripting"
)

// Plugin is a compiled template-variable plugin: a JavaScript file whose
// default-exported function is invoked with caller-provided arguments
// during template rendering. Arguments are passed in source order.
type Plugin struct {
	Name string
	Path string
	Func func(...any) (any, error)
}

// Plugins maps plugin names to compiled plugins. When plugins are merged
// or loaded in order, later entries replace earlier entries with the same
// name.
type Plugins map[string]Plugin

// pluginCallTimeout caps each variable-plugin invocation. The text/template
// rendering path has no native context plumbing, so each call gets its own
// background-derived timeout.
const pluginCallTimeout = 250 * time.Millisecond

// pluginNameRegexp matches valid Go-identifier-shaped names. text/template
// requires Go-identifier-shaped function names in a FuncMap.
var pluginNameRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// reservedPluginNames are identifiers that the variable DSL reserves.
var reservedPluginNames = map[string]struct{}{
	getFuncName:      {},
	textFuncName:     {},
	markdownFuncName: {},
}

// FuncMap returns the plugin as a single-entry text/template-compatible
// function map.
func (p Plugin) FuncMap() FuncMap {
	if p.Name == "" || p.Func == nil {
		return nil
	}
	return FuncMap{p.Name: p.Func}
}

// FuncMap returns the plugins as a text/template-compatible function map.
func (plugins Plugins) FuncMap() FuncMap {
	if len(plugins) == 0 {
		return nil
	}
	funcs := FuncMap{}
	for name, plugin := range plugins {
		if plugin.Func == nil {
			continue
		}
		funcs[name] = plugin.Func
	}
	return funcs
}

// LoadPlugin reads a JavaScript file from path, validates its name against
// the variable-function naming rules, and compiles its default-exported
// function into a callable Plugin.
func LoadPlugin(path string) (Plugin, error) {
	raw, err := scripting.LoadPlugin(path)
	if err != nil {
		return Plugin{}, err
	}
	if err := validatePluginName(raw.Name); err != nil {
		return Plugin{}, fmt.Errorf("variable: load %q: %w", path, err)
	}
	callable, err := raw.Compile(nil)
	if err != nil {
		return Plugin{}, fmt.Errorf("variable: compile %q: %w", path, err)
	}
	fn := func(args ...any) (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), pluginCallTimeout)
		defer cancel()
		return callable(ctx, nil, args...)
	}
	return Plugin{Name: raw.Name, Path: raw.Path, Func: fn}, nil
}

// LoadPlugins compiles a batch of template-variable plugin files. Later
// entries with the same derived name replace earlier ones.
func LoadPlugins(paths []string) (Plugins, error) {
	plugins := Plugins{}
	for _, path := range paths {
		plugin, err := LoadPlugin(path)
		if err != nil {
			return nil, err
		}
		plugins[plugin.Name] = plugin
	}
	return plugins, nil
}

// MergePlugins returns a new Plugins map with later groups taking precedence.
func MergePlugins(groups ...Plugins) Plugins {
	merged := Plugins{}
	for _, group := range groups {
		maps.Copy(merged, group)
	}
	return merged
}

func validatePluginName(name string) error {
	if !pluginNameRegexp.MatchString(name) {
		return fmt.Errorf("invalid plugin name %q (must match %s)", name, pluginNameRegexp.String())
	}
	if _, reserved := reservedPluginNames[name]; reserved {
		return fmt.Errorf("reserved plugin name %q", name)
	}
	return nil
}
