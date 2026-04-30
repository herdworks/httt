package scripting

import (
	"strings"
	"testing"
)

func TestPluginSelector(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		want    string
		wantErr string
	}{
		{
			name: "function declaration",
			source: `
				function upper(input) {
					return input.toUpperCase()
				}
				const plugin = upper
			`,
			want: "upper",
		},
		{
			name: "arrow function",
			source: `
				const upper = (input) => input.toUpperCase()
				const plugin = upper
			`,
			want: "upper",
		},
		{
			name: "helper function",
			source: `
				function helper(input) {
					return input.toUpperCase()
				}
				function upper(input) {
					return helper(input)
				}
				const plugin = upper
			`,
			want: "upper",
		},
		{
			name: "comments ignored",
			source: `
				// ht:plugin ignored
				function upper(input) {
					return input.toUpperCase()
				}
				const plugin = upper
			`,
			want: "upper",
		},
		{
			name: "missing selector",
			source: `
				function upper(input) {
					return input.toUpperCase()
				}
			`,
			wantErr: "missing plugin selector",
		},
		{
			name: "duplicate selector",
			source: `
				function upper(input) {
					return input.toUpperCase()
				}
				const plugin = upper
				const plugin = upper
			`,
			wantErr: "duplicate",
		},
		{
			name: "let selector",
			source: `
				function upper(input) {
					return input.toUpperCase()
				}
				let plugin = upper
			`,
			wantErr: "must be declared with const",
		},
		{
			name: "inline selector expression",
			source: `
				const plugin = (input) => input.toUpperCase()
			`,
			wantErr: "must reference a callable identifier",
		},
		{
			name: "unknown selector",
			source: `
				const plugin = upper
			`,
			wantErr: "does not refer to a top-level callable binding",
		},
		{
			name: "non-callable selector",
			source: `
				const upper = "not a function"
				const plugin = upper
			`,
			wantErr: "does not refer to a top-level callable binding",
		},
		{
			name: "reserved selector",
			source: `
				function get(input) {
					return input
				}
				const plugin = get
			`,
			wantErr: "reserved plugin name",
		},
		{
			name: "top-level return is invalid",
			source: `
				return input
				const plugin = upper
			`,
			wantErr: "parse plugin function",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PluginSelector(tc.source)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("PluginSelector() error = nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("PluginSelector() error = %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("PluginSelector() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("PluginSelector() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestScript_MetadataCommentCompilesAsScriptBody(t *testing.T) {
	fn, err := Script("// ht:plugin ignored\nreturn input.toUpperCase()", "")
	if err != nil {
		t.Fatalf("Script() error = %v", err)
	}

	got, err := fn("hello")
	if err != nil {
		t.Fatalf("compiled script error = %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("compiled script = %#v, want %#v", got, "HELLO")
	}
}
