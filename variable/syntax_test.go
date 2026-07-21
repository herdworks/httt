package variable

import (
	"errors"
	"strings"
	"testing"
)

func TestRewriteCSS_Basic(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
		funcs  FuncMap
	}{
		{
			name:   "double-quoted keypath",
			source: `--ht-value("theme.color")`,
			want:   `${ get "theme.color" }`,
		},
		{
			name:   "single-quoted keypath",
			source: `--ht-value('theme.color')`,
			want:   `${ get "theme.color" }`,
		},
		{
			name:   "fallback keypaths in body",
			source: `--ht-value("site.title, page.title")`,
			want:   `${ get "site.title, page.title" }`,
		},
		{
			name:   "inside CSS rule",
			source: `.hero { color: --ht-value("theme.color"); }`,
			want:   `.hero { color: ${ get "theme.color" }; }`,
		},
		{
			name:   "multiple calls in one rule",
			source: `.a { color: --ht-value("theme.color"); bg: --ht-value("theme.bg"); }`,
			want:   `.a { color: ${ get "theme.color" }; bg: ${ get "theme.bg" }; }`,
		},
		{
			name:   "whitespace around argument",
			source: `--ht-value(  "x"  )`,
			want:   `${ get "x" }`,
		},
		{
			name:   "multiline whitespace",
			source: "--ht-value(\n\t\"x\"\n)",
			want:   `${ get "x" }`,
		},
		{
			name:   "at position 0",
			source: `--ht-value("x")`,
			want:   `${ get "x" }`,
		},
		{
			name:   "unicode keypath",
			source: `--ht-value("café.title")`,
			want:   `${ get "café.title" }`,
		},
		{
			name:   "no calls passes through",
			source: `.foo { color: red; }`,
			want:   `.foo { color: red; }`,
		},
		{
			name:   "second argument fallback with pipeline",
			source: `--ht-value("page.title | upper", "DEFAULT")`,
			want:   `${ get "page.title" | upper | cssDefault "DEFAULT" }`,
			funcs:  FuncMap{"upper": strings.ToUpper},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewriteWithFuncs(tc.source, SyntaxCSS, tc.funcs)
			if err != nil {
				t.Fatalf("rewrite() error = %v", err)
			}

			if got != tc.want {
				t.Fatalf("rewrite() mismatch:\nwant: %s\n got: %s", tc.want, got)
			}
		})
	}
}

func TestRewriteCSS_Pipeline(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
		funcs  FuncMap
	}{
		{
			name:   "pipeline tail with fallback",
			source: `--ht-value("page.title, \"Untitled\" | upper")`,
			want:   `${ get "page.title" "Untitled" | upper }`,
			funcs:  FuncMap{"upper": strings.ToUpper},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewriteWithFuncs(tc.source, SyntaxCSS, tc.funcs)
			if err != nil {
				t.Fatalf("rewrite() error = %v", err)
			}

			if got != tc.want {
				t.Fatalf("rewrite() mismatch:\nwant: %s\n got: %s", tc.want, got)
			}
		})
	}
}

func TestRewriteCSS_Fallback(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "string literal fallback",
			source: `--ht-value("theme.color, \"red\"")`,
			want:   `${ theme.color, "red" }`,
		},
		{
			name:   "unquoted second argument fallback",
			source: `--ht-value("theme.color", red)`,
			want:   `${ theme.color | cssDefault "red" }`,
		},
		{
			name:   "quoted second argument fallback",
			source: `--ht-value("theme.color", "red")`,
			want:   `${ theme.color | cssDefault "red" }`,
		},
		{
			name:   "single-quoted second argument fallback",
			source: `--ht-value("theme.color", 'red')`,
			want:   `${ theme.color | cssDefault "red" }`,
		},
		{
			name:   "fallback with internal commas and parens",
			source: `--ht-value("theme.color, \"rgb(255, 0, 0)\"")`,
			want:   `${ theme.color, "rgb(255, 0, 0)" }`,
		},
		{
			name:   "second argument fallback with internal commas and parens",
			source: `--ht-value("theme.color", rgba(0, 0, 0, 0.5))`,
			want:   `${ theme.color | cssDefault "rgba(0, 0, 0, 0.5)" }`,
		},
		{
			name:   "quoted fallback with internal comma",
			source: `--ht-value("page.font, \"Arial, sans-serif\"")`,
			want:   `${ page.font, "Arial, sans-serif" }`,
		},
		{
			name:   "fallback with pipeline body",
			source: `--ht-value("page.title, \"DEFAULT\" | upper")`,
			want:   `${ page.title, "DEFAULT" | upper }`,
		},
		{
			name:   "second argument fallback with pipeline body",
			source: `--ht-value("page.title | upper", "DEFAULT")`,
			want:   `${ page.title | upper | cssDefault "DEFAULT" }`,
		},
		{
			name:   "CSS length value fallback",
			source: `.a { padding: --ht-value("theme.pad, \"10px\""); }`,
			want:   `.a { padding: ${ theme.pad, "10px" }; }`,
		},
		{
			name:   "CSS length second argument fallback",
			source: `.a { padding: --ht-value("theme.pad", 10px); }`,
			want:   `.a { padding: ${ theme.pad | cssDefault "10px" }; }`,
		},
		{
			name:   "explicit empty-string fallback",
			source: `--ht-value("theme.color, \"\"")`,
			want:   `${ theme.color, "" }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewriteCSS(tc.source)
			if err != nil {
				t.Fatalf("rewriteCSS() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("rewriteCSS() mismatch:\nwant: %q\n got: %q", tc.want, got)
			}
		})
	}
}

func TestRewriteCSS_Escapes(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "escaped double quote inside double-quoted",
			source: `--ht-value("printf \"%s\" page.title")`,
			want:   `${ printf "%s" page.title }`,
		},
		{
			name:   "escaped single quote inside single-quoted",
			source: `--ht-value('it\'s')`,
			want:   `${ it's }`,
		},
		{
			name:   "escaped backslash",
			source: `--ht-value("a\\b")`,
			want:   `${ a\b }`,
		},
		{
			name:   "unknown escape passes through",
			source: `--ht-value("a\nb")`,
			want:   `${ a\nb }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewriteCSS(tc.source)
			if err != nil {
				t.Fatalf("rewriteCSS() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("rewriteCSS() mismatch:\nwant: %q\n got: %q", tc.want, got)
			}
		})
	}
}

func TestRewriteCSS_WordBoundary(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "identifier prefix passes through",
			source: `my--ht-value("x")`,
			want:   `my--ht-value("x")`,
		},
		{
			name:   "extra hyphen prefix passes through",
			source: `---ht-value("x")`,
			want:   `---ht-value("x")`,
		},
		{
			name:   "underscore prefix passes through",
			source: `_--ht-value("x")`,
			want:   `_--ht-value("x")`,
		},
		{
			name:   "dot prefix is a boundary",
			source: `.--ht-value("x")`,
			want:   `.${ get "x" }`,
		},
		{
			name:   "colon prefix is a boundary",
			source: `color:--ht-value("x")`,
			want:   `color:${ get "x" }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewrite(tc.source, SyntaxCSS)
			if err != nil {
				t.Fatalf("rewrite() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("rewrite() mismatch:\nwant: %q\n got: %q", tc.want, got)
			}
		})
	}
}

func TestRewriteCSS_CaseSensitive(t *testing.T) {
	source := `--HT-VALUE("x"); --Ht-Value("y")`
	got, err := rewriteCSS(source)
	if err != nil {
		t.Fatalf("rewriteCSS() error = %v", err)
	}
	if got != source {
		t.Fatalf("rewriteCSS() should leave non-lowercase untouched:\nwant: %q\n got: %q", source, got)
	}
}

func TestRewriteCSS_PassesThroughDollarBrace(t *testing.T) {
	source := `.a { content: "${x}"; color: --ht-value("theme.color"); }`
	got, err := rewriteCSS(source)
	if err != nil {
		t.Fatalf("rewriteCSS() error = %v", err)
	}
	want := `.a { content: "${x}"; color: ${ theme.color }; }`
	if got != want {
		t.Fatalf("rewriteCSS() mismatch:\nwant: %q\n got: %q", want, got)
	}
}

func TestRewriteCSS_Rejects(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		contains string
	}{
		{
			name:     "empty double-quoted",
			source:   `--ht-value("")`,
			contains: "empty template argument",
		},
		{
			name:     "whitespace-only template arg",
			source:   `--ht-value("   ")`,
			contains: "empty template argument",
		},
		{
			name:     "bareword template argument",
			source:   `--ht-value(x)`,
			contains: "expected quoted argument",
		},
		{
			name:     "keypath without quotes",
			source:   `--ht-value(page.title)`,
			contains: "expected quoted argument",
		},
		{
			name:     "unterminated double-quoted string",
			source:   `--ht-value("x`,
			contains: "unterminated string",
		},
		{
			name:     "unterminated single-quoted string",
			source:   `--ht-value('x`,
			contains: "unterminated string",
		},
		{
			name:     "missing close paren",
			source:   `--ht-value("x"`,
			contains: "missing closing )",
		},
		{
			name:     "junk before close paren",
			source:   `--ht-value("x" y)`,
			contains: "expected , or )",
		},
		{
			name:     "open paren alone",
			source:   `--ht-value(`,
			contains: "end of input",
		},
		{
			name:     "empty fallback",
			source:   `--ht-value("x", )`,
			contains: "empty fallback value",
		},
		{
			name:     "trailing comma with nothing after",
			source:   `--ht-value("x",`,
			contains: "missing fallback value",
		},
		{
			name:     "unterminated fallback string",
			source:   `--ht-value("x", "red`,
			contains: "unterminated string",
		},
		{
			name:     "unterminated unquoted fallback",
			source:   `--ht-value("x", rgb(255, 0, 0)`,
			contains: "missing closing )",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rewriteCSS(tc.source)
			if err == nil {
				t.Fatal("rewriteCSS() error = nil, want error")
			}
			if !errors.Is(err, errInvalidHTValueCall) {
				t.Fatalf("rewriteCSS() error = %v, want errInvalidHTValueCall", err)
			}
			if !errors.Is(err, errInvalidValue) {
				t.Fatalf("rewriteCSS() error should wrap errInvalidValue, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("rewriteCSS() error missing substring %q: %v", tc.contains, err)
			}
		})
	}
}

func TestRewriteCSS_InheritsKeypathValidation(t *testing.T) {
	_, err := rewrite(`--ht-value("if page.title")`, SyntaxCSS)
	if err == nil {
		t.Fatal("rewrite() error = nil, want function-not-registered error")
	}
	if !strings.Contains(err.Error(), `function "if" not registered`) {
		t.Fatalf("rewrite() error = %v, want function-not-registered error", err)
	}
}

func TestRewriteCSS_InheritsCommentRejection(t *testing.T) {
	_, err := rewrite(`--ht-value("/* hidden */")`, SyntaxCSS)
	if err == nil {
		t.Fatal("rewrite() error = nil, want template-comment error")
	}
	if !strings.Contains(err.Error(), errTemplateComment.Error()) {
		t.Fatalf("rewrite() error = %v, want substring %q", err, errTemplateComment.Error())
	}
}

func TestRenderCSS_EndToEnd(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{
			"color": "red",
		},
	}

	source := `.hero { color: --ht-value("theme.color"); }`
	rendered, err := RenderString(source, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	want := `.hero { color: red; }`
	if rendered != want {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackWhenNil(t *testing.T) {
	data := testGetter{}

	rendered, err := RenderString(`.a { color: --ht-value("theme.color", red); }`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	want := `.a { color: red; }`
	if rendered != want {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackWhenEmptyString(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{
			"color": "",
		},
	}

	rendered, err := RenderString(`.a { color: --ht-value("theme.color", red); }`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	want := `.a { color: ; }`
	if rendered != want {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackWhenFunctionReturnsNil(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{
			"color": "blue",
		},
	}
	funcs := FuncMap{
		"maybe": func(any) any {
			return nil
		},
	}

	rendered, err := RenderString(`.a { color: --ht-value("maybe theme.color", red); }`, data, SyntaxCSS, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	want := `.a { color: red; }`
	if rendered != want {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackSkippedWhenFunctionReturnsEmptyString(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{
			"color": "blue",
		},
	}
	funcs := FuncMap{
		"maybe": func(any) string {
			return ""
		},
	}

	rendered, err := RenderString(`.a { color: --ht-value("maybe theme.color", red); }`, data, SyntaxCSS, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	want := `.a { color: ; }`
	if rendered != want {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackSkippedWhenPresent(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{
			"color": "blue",
		},
	}

	rendered, err := RenderString(`.a { color: --ht-value("theme.color", red); }`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	want := `.a { color: blue; }`
	if rendered != want {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackRGB(t *testing.T) {
	data := testGetter{}

	rendered, err := RenderString(`.a { color: --ht-value("theme.color", rgb(255, 0, 0)); }`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	want := `.a { color: rgb(255, 0, 0); }`
	if rendered != want {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackResolution(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Page Title",
		},
	}

	rendered, err := RenderString(`--ht-value("site.title, page.title")`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "Page Title" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Page Title", rendered)
	}
}

func TestValidateCSS_PropagatesRewriterErrors(t *testing.T) {
	err := validate(`--ht-value(x)`, SyntaxCSS)
	if err == nil {
		t.Fatal("validate() error = nil, want error")
	}
	if !errors.Is(err, errInvalidHTValueCall) {
		t.Fatalf("validate() error should wrap errInvalidHTValueCall, got %v", err)
	}
}

func TestParseCSS_NormalizedExposesRewritten(t *testing.T) {
	parsed, err := parse(`--ht-value("page.title")`, SyntaxCSS)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if parsed.source != `--ht-value("page.title")` {
		t.Fatalf("source mismatch:\nwant: %q\n got: %q", `--ht-value("page.title")`, parsed.source)
	}

	if parsed.normalized != `${ get "page.title" }` {
		t.Fatalf("normalized mismatch:\nwant: %q\n got: %q", `${ get "page.title" }`, parsed.normalized)
	}
}

func TestRewriteCSS_DefaultModeIgnoresCalls(t *testing.T) {
	source := `.a { color: --ht-value("theme.color"); }`
	got, err := rewrite(source, SyntaxDefault)
	if err != nil {
		t.Fatalf("rewrite() error = %v", err)
	}
	if got != source {
		t.Fatalf("rewrite() in SyntaxDefault should leave --ht-value(...) untouched:\nwant: %q\n got: %q", source, got)
	}
}

func TestHTFallback_InDefaultSyntax(t *testing.T) {
	data := testGetter{}
	rendered, err := RenderString(`${ page.title, "Untitled" }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Untitled", rendered)
	}
}

func TestRenderCSS_MissingErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "double-quoted bare call", source: `--ht-value("theme.color")`},
		{name: "single-quoted bare call", source: `--ht-value('theme.color')`},
		{name: "inside CSS rule", source: `.a { color: --ht-value("theme.color"); }`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RenderString(tc.source, testGetter{}, SyntaxCSS)
			if err == nil {
				t.Fatal("RenderString() error = nil, want MissingValueError")
			}
			var miss *MissingValueError
			if !errors.As(err, &miss) {
				t.Fatalf("RenderString() error = %v, want MissingValueError", err)
			}
		})
	}
}

func TestRenderCSS_FallbackOnMissPreventsRestore(t *testing.T) {
	rendered, err := RenderString(`--ht-value("theme.color", red)`, testGetter{}, SyntaxCSS)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "red" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "red", rendered)
	}
}

func TestRenderCSS_MultiActionAnyMissErrors(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{"color": "red"},
	}
	source := `.a { color: --ht-value("theme.color"); bg: --ht-value("theme.bg"); }`

	_, err := RenderString(source, data, SyntaxCSS)
	if err == nil {
		t.Fatal("RenderString() error = nil, want MissingValueError")
	}
	var miss *MissingValueError
	if !errors.As(err, &miss) {
		t.Fatalf("RenderString() error = %v, want MissingValueError", err)
	}
}
