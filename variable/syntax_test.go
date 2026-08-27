package variable

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRenderCSS(t *testing.T) {
	data := testGetter{"theme": map[string]any{"color": "red", "empty": ""}}
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "keypath", source: `.a { color: --ht-var("theme.color"); }`, want: `.a { color: red; }`},
		{name: "missing fallback", source: `--ht-var("theme.missing", blue)`, want: "blue"},
		{name: "quoted fallback", source: `--ht-var('theme.missing', 'rgb(1, 2, 3)')`, want: "rgb(1, 2, 3)"},
		{name: "empty uses fallback", source: `--ht-var("theme.empty", blue)`, want: "blue"},
		{name: "multiple", source: `--ht-var("theme.color")/--ht-var("theme.missing", x)`, want: "red/x"},
		{name: "non-boundary", source: `x--ht-var("theme.color")`, want: `x--ht-var("theme.color")`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := RenderString(tc.source, data, SyntaxCSS)
			if err != nil {
				t.Fatalf("RenderString() error = %v", err)
			}
			if rendered != tc.want {
				t.Fatalf("RenderString() = %q, want %q", rendered, tc.want)
			}
		})
	}
}

func TestRenderCSSFunctionCallAndFallback(t *testing.T) {
	funcs := FuncMap{"optional": func(value any) any { return value }}
	rendered, err := RenderString(`--ht-var("optional(theme.color)", DEFAULT)`, testGetter{}, SyntaxCSS, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "DEFAULT" {
		t.Fatalf("RenderString() = %q, want DEFAULT", rendered)
	}
}

func TestRenderCSSFunctionCallOptionsEnvelope(t *testing.T) {
	var got map[string]any
	funcs := FuncMap{
		"optional": func(envelope map[string]any) string {
			got = envelope
			return "blue"
		},
	}
	rendered, err := RenderString(`--ht-var("optional(theme.color, { \"verbose\": true })")`, testGetter{"theme": map[string]any{"color": "red"}}, SyntaxCSS, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "blue" {
		t.Fatalf("RenderString() = %q, want blue", rendered)
	}
	if want := []any{"red"}; !reflect.DeepEqual(got["args"], want) {
		t.Fatalf("envelope args = %#v, want %#v", got["args"], want)
	}
	if options := got["options"].(map[string]any); options["verbose"] != true {
		t.Fatalf("envelope options = %#v", options)
	}
}

func TestRenderCSSRejectsPipes(t *testing.T) {
	funcs := FuncMap{"upper": strings.ToUpper}
	_, err := RenderString(`--ht-var("theme.color | upper")`, testGetter{}, SyntaxCSS, funcs)
	if err == nil || !strings.Contains(err.Error(), "pipes are not supported") {
		t.Fatalf("RenderString() error = %v, want pipe rejection", err)
	}
}

func TestRenderCSSRejectsMalformedCalls(t *testing.T) {
	tests := []string{
		`--ht-var(x)`,
		`--ht-var("")`,
		`--ht-var("x"`,
		`--ht-var("x",)`,
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			_, err := RenderString(source, testGetter{}, SyntaxCSS)
			if err == nil {
				t.Fatal("RenderString() error = nil")
			}
			if !errors.Is(err, errInvalidHTVarCall) {
				t.Fatalf("RenderString() error = %v, want errInvalidHTVarCall", err)
			}
		})
	}
}

func TestDefaultSyntaxLeavesCSSCallsLiteral(t *testing.T) {
	source := `.a { color: --ht-var("theme.color"); }`
	rendered, err := RenderString(source, testGetter{}, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != source {
		t.Fatalf("RenderString() = %q, want unchanged source", rendered)
	}
}

func TestCSSSyntaxLeavesLegacyCallsLiteral(t *testing.T) {
	source := `.a { color: --ht-value("theme.color"); }`
	rendered, err := RenderString(source, testGetter{"theme": map[string]any{"color": "red"}}, SyntaxCSS)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != source {
		t.Fatalf("RenderString() = %q, want unchanged legacy source", rendered)
	}
}
