package variable

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	texttemplate "text/template"
)

type testGetter map[string]any

func (data testGetter) Get(keypath string) any {
	value := any(map[string]any(data))
	for _, key := range strings.Split(keypath, ".") {
		mapped, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = mapped[key]
	}
	return value
}

func TestRenderStringImplicitAndExplicitGet(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": "Page"}}
	for _, source := range []string{
		`${ page.missing, page.title, "Fallback" }`,
		`${ get(page.missing, page.title, "Fallback") }`,
	} {
		rendered, err := RenderString(source, data, SyntaxDefault)
		if err != nil {
			t.Fatalf("RenderString(%q) error = %v", source, err)
		}
		if rendered != "Page" {
			t.Fatalf("RenderString(%q) = %q, want Page", source, rendered)
		}
	}
}

func TestRenderStringFallbackSelectsFirstNonEmptyDataValue(t *testing.T) {
	values := []struct {
		name  string
		value any
		want  string
	}{
		{name: "false", value: false, want: "false"},
		{name: "zero", value: 0, want: "0"},
		{name: "empty string", value: "", want: "fallback"},
		{name: "empty slice", value: []any{}, want: "[]"},
		{name: "empty map", value: map[string]any{}, want: "{}"},
	}
	for _, tc := range values {
		t.Run(tc.name, func(t *testing.T) {
			data := testGetter{"page": map[string]any{"value": tc.value}}
			rendered, err := RenderString(`${ page.value, "fallback" }`, data, SyntaxDefault)
			if err != nil {
				t.Fatalf("RenderString() error = %v", err)
			}
			if rendered != tc.want {
				t.Fatalf("RenderString() = %q, want %q", rendered, tc.want)
			}
		})
	}
}

func TestRenderStringAcceptsEmptyLiteralFallback(t *testing.T) {
	rendered, err := RenderString(`${ page.value, "" }`, testGetter{
		"page": map[string]any{"value": ""},
	}, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "" {
		t.Fatalf("RenderString() = %q, want empty string", rendered)
	}
}

func TestRenderStringRejectsEmptyTemplateDataWithoutFallback(t *testing.T) {
	_, err := RenderString(`${ page.value }`, testGetter{
		"page": map[string]any{"value": ""},
	}, SyntaxDefault)
	if err == nil {
		t.Fatal("RenderString() error = nil, want MissingValueError")
	}
	var missing *MissingValueError
	if !errors.As(err, &missing) {
		t.Fatalf("RenderString() error = %T %v, want MissingValueError", err, err)
	}
}

func TestRenderStringPluginArgumentsUseGetterAndSourceOrder(t *testing.T) {
	getter := &recordingGetter{values: map[string]any{
		"page.first": "first",
		"page.zero":  0,
	}}
	var got []any
	funcs := FuncMap{
		"example": func(arguments ...any) string {
			got = append(got, arguments...)
			return "ok"
		},
	}

	rendered, err := RenderString(
		`${ example(page.first, page.missing, page.zero, "literal", false, 2.5) }`,
		getter,
		SyntaxDefault,
		funcs,
	)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "ok" {
		t.Fatalf("RenderString() = %q, want ok", rendered)
	}
	wantArguments := []any{"first", nil, 0, "literal", false, 2.5}
	if !reflect.DeepEqual(got, wantArguments) {
		t.Fatalf("plugin arguments = %#v, want %#v", got, wantArguments)
	}
	wantCalls := []string{"page.first", "page.missing", "page.zero"}
	if !reflect.DeepEqual(getter.calls, wantCalls) {
		t.Fatalf("Getter calls = %v, want %v", getter.calls, wantCalls)
	}
}

func TestRenderStringBuiltinsUseFallbackSelection(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": "<Hello>"}}
	funcs := FuncMap{
		"text": texttemplate.HTMLEscaper,
		"markdown": func(value any) (string, error) {
			return "md:" + fmt.Sprint(value), nil
		},
	}
	tests := []struct {
		source string
		want   string
	}{
		{source: `${ text(page.missing, page.title, "fallback") }`, want: "&lt;Hello&gt;"},
		{source: `${ markdown(page.missing, "# Placeholder") }`, want: "md:# Placeholder"},
	}
	for _, tc := range tests {
		rendered, err := RenderString(tc.source, data, SyntaxDefault, funcs)
		if err != nil {
			t.Fatalf("RenderString(%q) error = %v", tc.source, err)
		}
		if rendered != tc.want {
			t.Fatalf("RenderString(%q) = %q, want %q", tc.source, rendered, tc.want)
		}
	}
}

func TestRenderStringJSONEncoding(t *testing.T) {
	data := testGetter{"value": map[string]any{
		"map":    map[string]any{"b": 2, "a": true},
		"slice":  []any{"x", 1, false},
		"number": json.Number("5"),
	}}
	tests := []struct {
		source string
		want   string
	}{
		{source: `${ value.map }`, want: `{"a":true,"b":2}`},
		{source: `${ value.slice }`, want: `["x",1,false]`},
		{source: `${ value.number }`, want: `5`},
	}
	for _, tc := range tests {
		rendered, err := RenderString(tc.source, data, SyntaxDefault)
		if err != nil {
			t.Fatalf("RenderString(%q) error = %v", tc.source, err)
		}
		if rendered != tc.want {
			t.Fatalf("RenderString(%q) = %q, want %q", tc.source, rendered, tc.want)
		}
	}
}

func TestRenderStringJSONEncodesPluginResult(t *testing.T) {
	funcs := FuncMap{
		"object": func(value any) map[string]any {
			return map[string]any{"value": value, "ok": true}
		},
	}
	rendered, err := RenderString(
		`${ object(page.title) }`,
		testGetter{"page": map[string]any{"title": "Hello"}},
		SyntaxDefault,
		funcs,
	)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != `{"ok":true,"value":"Hello"}` {
		t.Fatalf("RenderString() = %q", rendered)
	}
}

func TestRenderStringJSONErrors(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	data := testGetter{"value": map[string]any{
		"nan":     math.NaN(),
		"channel": make(chan int),
		"cycle":   cycle,
	}}
	for _, source := range []string{`${ value.nan }`, `${ value.channel }`, `${ value.cycle }`} {
		if _, err := RenderString(source, data, SyntaxDefault); err == nil {
			t.Fatalf("RenderString(%q) error = nil", source)
		}
	}
}

func TestRenderStringMissingValue(t *testing.T) {
	source := `${ page.missing }`
	_, err := RenderString(source, testGetter{}, SyntaxDefault)
	var missing *MissingValueError
	if !errors.As(err, &missing) {
		t.Fatalf("RenderString() error = %v, want MissingValueError", err)
	}
	if missing.Source != source {
		t.Fatalf("MissingValueError.Source = %q, want %q", missing.Source, source)
	}
}

func TestRenderStringLiteralTextAndTrimMarkers(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": "Title"}}
	rendered, err := RenderString("before \n ${- page.title -} \n after", data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "beforeTitleafter" {
		t.Fatalf("RenderString() = %q, want beforeTitleafter", rendered)
	}

	rendered, err = RenderString(`${ page.missing, "a|b" } | literal`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() with literal pipe error = %v", err)
	}
	if rendered != "a|b | literal" {
		t.Fatalf("RenderString() = %q, want %q", rendered, "a|b | literal")
	}
}

func TestRenderStringRejectsUnsupportedSyntax(t *testing.T) {
	funcs := FuncMap{"upper": strings.ToUpper}
	tests := []struct {
		name   string
		source string
	}{
		{name: "pipe", source: `${ page.title | upper }`},
		{name: "nested call", source: `${ upper(get(page.title)) }`},
		{name: "grouping", source: `${ upper((page.title)) }`},
		{name: "trailing comma", source: `${ upper(page.title,) }`},
		{name: "unknown function", source: `${ unknown(page.title) }`},
		{name: "raw template command", source: `${ upper page.title }`},
		{name: "array literal", source: `${ upper([page.title]) }`},
		{name: "object literal", source: `${ upper({value: page.title}) }`},
		{name: "null", source: `${ upper(null) }`},
		{name: "undefined", source: `${ upper(undefined) }`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RenderString(tc.source, testGetter{}, SyntaxDefault, funcs); err == nil {
				t.Fatalf("RenderString(%q) error = nil", tc.source)
			}
		})
	}
}

func TestRenderStringFunctionErrorsPropagate(t *testing.T) {
	want := errors.New("boom")
	funcs := FuncMap{"explode": func(any) (any, error) { return nil, want }}
	_, err := RenderString(`${ explode(page.title) }`, testGetter{}, SyntaxDefault, funcs)
	if !errors.Is(err, want) {
		t.Fatalf("RenderString() error = %v, want wrapped boom", err)
	}
}
