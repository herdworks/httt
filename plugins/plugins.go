package plugins

import (
	"fmt"
	"os"
	texttemplate "text/template"

	"github.com/herdworks/hypertemplates-go/scripting"
)

// FuncMap registers plugin functions with text/template-compatible renderers.
type FuncMap = texttemplate.FuncMap

// Func is the executable function shape for HyperTemplates plugins.
type Func func(...any) (any, error)

// Plugin is a compiled HyperTemplates plugin.
//
// Plugins are JavaScript-backed value transformers compiled into Go functions
// with this signature:
//
//	func(...any) (any, error)
//
// That function shape lets value pipelines pass explicit arguments first and
// the upstream pipeline value last while preserving normal text/template error
// propagation.
type Plugin struct {
	Name string
	Path string
	Func Func
}

// FuncMap returns the plugin as a text/template-compatible function map.
func (plugin Plugin) FuncMap() FuncMap {
	if plugin.Name == "" || plugin.Func == nil {
		return nil
	}
	return FuncMap{plugin.Name: plugin.Func}
}

// Plugins maps plugin names to compiled plugins. When plugins are loaded or
// merged in order, later plugins with the same name replace earlier plugins.
type Plugins map[string]Plugin

// FuncMap returns the plugins as text/template-compatible functions.
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

// LoadPlugin compiles one scripting-backed plugin from a local file.
func LoadPlugin(path string) (Plugin, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Plugin{}, fmt.Errorf("read plugin %q: %w", path, err)
	}
	source := string(src)

	name, fn, err := scripting.Function(source)
	if err != nil {
		return Plugin{}, fmt.Errorf("compile plugin %q: %w", path, err)
	}
	return Plugin{Name: name, Path: path, Func: Func(fn)}, nil
}

// LoadPlugins compiles scripting-backed plugins from local files.
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

// Merge returns a new plugin map with later groups taking precedence.
func Merge(groups ...Plugins) Plugins {
	merged := Plugins{}
	for _, group := range groups {
		for name, plugin := range group {
			merged[name] = plugin
		}
	}
	return merged
}
