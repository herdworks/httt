package scripting

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestPluginError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *PluginError
		want string
	}{
		{
			name: "with path",
			err:  &PluginError{Feature: "require", Path: "plugins/foo.js"},
			want: "PluginError: require is not supported in plugins yet (in plugins/foo.js)",
		},
		{
			name: "without path",
			err:  &PluginError{Feature: "import"},
			want: "PluginError: import is not supported in plugins yet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPluginError_AsThroughLoadTimeError(t *testing.T) {
	src := `import foo from './foo.js'` + "\n" + `export default function() { return foo }`
	plugin := Plugin{Name: "x", Source: src, Path: "x.js"}
	_, err := plugin.Compile(nil)
	if err == nil {
		t.Fatal("Compile error = nil, want load-time PluginError")
	}
	var perr *PluginError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %v, want errors.As to *PluginError", err)
	}
	if perr.Feature != "import" {
		t.Fatalf("Feature = %q, want import", perr.Feature)
	}
	if perr.Path != "x.js" {
		t.Fatalf("Path = %q, want x.js", perr.Path)
	}
}

func TestPluginError_AsThroughRuntimeError(t *testing.T) {
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
		t.Fatal("Call error = nil, want runtime PluginError")
	}
	var perr *PluginError
	if !errors.As(err, &perr) {
		t.Fatalf("err = %v (%T), want errors.As to *PluginError", err, err)
	}
	if perr.Feature != "require" {
		t.Fatalf("Feature = %q, want require", perr.Feature)
	}
	// Path should be propagated from the Plugin.Path at Compile time.
	if perr.Path != "x.js" {
		t.Fatalf("Path = %q, want x.js", perr.Path)
	}
	// And the message should still match the human-readable form.
	if !strings.Contains(perr.Error(), "require is not supported") {
		t.Fatalf("Error() = %q, want substring %q", perr.Error(), "require is not supported")
	}
}
