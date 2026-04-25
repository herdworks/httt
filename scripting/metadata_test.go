package scripting

import (
	"strings"
	"testing"
)

func TestPluginName(t *testing.T) {
	cases := []struct {
		name      string
		source    string
		want      string
		wantFound bool
		wantErr   string
	}{
		{
			name:      "line comment with space",
			source:    "// ht:plugin upper\ninput.toUpperCase()",
			want:      "upper",
			wantFound: true,
		},
		{
			name:      "line comment with colon",
			source:    "// ht:plugin: title\ninput",
			want:      "title",
			wantFound: true,
		},
		{
			name:      "line comment with equals",
			source:    "// ht:plugin=slug\ninput",
			want:      "slug",
			wantFound: true,
		},
		{
			name: "block comment",
			source: `/*
 * ht:plugin excerpt
 */
input`,
			want:      "excerpt",
			wantFound: true,
		},
		{
			name:      "at prefix",
			source:    "/** @ht:plugin upper */\ninput",
			want:      "upper",
			wantFound: true,
		},
		{
			name:      "not leading",
			source:    "input\n// ht:plugin upper",
			wantFound: false,
		},
		{
			name:      "missing",
			source:    "// ordinary comment\ninput",
			wantFound: false,
		},
		{
			name:    "invalid name",
			source:  "// ht:plugin upper-case\ninput",
			wantErr: "invalid plugin name",
		},
		{
			name:    "duplicate",
			source:  "// ht:plugin upper\n// ht:plugin title\ninput",
			wantErr: "duplicate plugin name",
		},
		{
			name:    "unterminated block comment",
			source:  "/* ht:plugin upper",
			wantErr: "unterminated metadata comment",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, found, err := PluginName(tc.source)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatal("PluginName() error = nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("PluginName() error = %v, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("PluginName() error = %v", err)
			}
			if found != tc.wantFound {
				t.Fatalf("PluginName() found = %v, want %v", found, tc.wantFound)
			}
			if got != tc.want {
				t.Fatalf("PluginName() name = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPluginName_MetadataCommentCompilesAsExpression(t *testing.T) {
	fn, err := Function("// ht:plugin upper\ninput.toUpperCase()", "")
	if err != nil {
		t.Fatalf("Function() error = %v", err)
	}

	got, err := fn("hello")
	if err != nil {
		t.Fatalf("compiled function error = %v", err)
	}
	if got != "HELLO" {
		t.Fatalf("compiled function = %#v, want %#v", got, "HELLO")
	}
}
