package scripting

import (
	"errors"
	"fmt"

	"github.com/dop251/goja"
)

// PluginError indicates that a plugin used a JavaScript feature that the
// HyperTemplates plugin runtime does not (yet) support — for example,
// `import` declarations or `require()` calls. PluginError surfaces from
// load-time validation (Go-side, e.g. the import pre-scan) and from runtime
// evaluation (JS-side, e.g. a stubbed `require()` being invoked).
type PluginError struct {
	// Feature is a short identifier for the unsupported feature, e.g.
	// "import", "require", "top-level-await".
	Feature string

	// Path is the plugin file path, when known. Optional.
	Path string
}

func (e *PluginError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("PluginError: %s is not supported in plugins yet (in %s)", e.Feature, e.Path)
	}
	return fmt.Sprintf("PluginError: %s is not supported in plugins yet", e.Feature)
}

// asPluginError returns a *PluginError if err originated from a JS exception
// thrown by the plugin wrapper's stubs (with name === "PluginError"), or nil
// otherwise. The Path field is populated from the path argument so callers
// can supply per-plugin context that the JS side does not have.
func asPluginError(err error, path string) *PluginError {
	if err == nil {
		return nil
	}

	var jsExc *goja.Exception
	if !errors.As(err, &jsExc) {
		return nil
	}

	val := jsExc.Value()
	if val == nil {
		return nil
	}

	obj, ok := val.(*goja.Object)
	if !ok {
		// Could be a string or other primitive thrown — not our shape.
		// Fall through to defensive Export() check.
		exported := val.Export()
		m, ok := exported.(map[string]any)
		if !ok {
			return nil
		}
		name, _ := m["name"].(string)
		if name != "PluginError" {
			return nil
		}
		feature, _ := m["pluginErrorFeature"].(string)
		return &PluginError{Feature: feature, Path: path}
	}

	if name := obj.Get("name"); name == nil || name.String() != "PluginError" {
		return nil
	}

	feature := ""
	if f := obj.Get("pluginErrorFeature"); f != nil && !goja.IsUndefined(f) && !goja.IsNull(f) {
		feature = f.String()
	}

	return &PluginError{Feature: feature, Path: path}
}
