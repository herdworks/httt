package format

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
	}{
		{
			name:   "double-quoted keypath",
			source: `--ht-fmt("theme.color")`,
			want:   `${ httd "theme.color" }`,
		},
		{
			name:   "single-quoted keypath",
			source: `--ht-fmt('theme.color')`,
			want:   `${ httd "theme.color" }`,
		},
		{
			name:   "fallback keypaths in body",
			source: `--ht-fmt("site.title, page.title")`,
			want:   `${ httd "site.title, page.title" }`,
		},
		{
			name:   "inside CSS rule",
			source: `.hero { color: --ht-fmt("theme.color"); }`,
			want:   `.hero { color: ${ httd "theme.color" }; }`,
		},
		{
			name:   "multiple calls in one rule",
			source: `.a { color: --ht-fmt("theme.color"); bg: --ht-fmt("theme.bg"); }`,
			want:   `.a { color: ${ httd "theme.color" }; bg: ${ httd "theme.bg" }; }`,
		},
		{
			name:   "whitespace around argument",
			source: `--ht-fmt(  "x"  )`,
			want:   `${ httd "x" }`,
		},
		{
			name:   "multiline whitespace",
			source: "--ht-fmt(\n\t\"x\"\n)",
			want:   `${ httd "x" }`,
		},
		{
			name:   "at position 0",
			source: `--ht-fmt("x")`,
			want:   `${ httd "x" }`,
		},
		{
			name:   "unicode keypath",
			source: `--ht-fmt("café.title")`,
			want:   `${ httd "café.title" }`,
		},
		{
			name:   "no calls passes through",
			source: `.foo { color: red; }`,
			want:   `.foo { color: red; }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewrite(tc.source, SyntaxCSS)
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
	}{
		{
			name:   "pipeline tail with default",
			source: `--ht-fmt("page.title | default \"Untitled\"")`,
			want:   `${ httd "page.title" | default "Untitled" }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewrite(tc.source, SyntaxCSS)
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
			name:   "unquoted identifier fallback",
			source: `--ht-fmt("theme.color", red)`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "quoted fallback",
			source: `--ht-fmt("theme.color", "red")`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "single-quoted fallback",
			source: `--ht-fmt("theme.color", 'red')`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "fallback with internal commas and parens",
			source: `--ht-fmt("theme.color", rgb(255, 0, 0))`,
			want:   `${ theme.color | default "rgb(255, 0, 0)" }`,
		},
		{
			name:   "quoted fallback with internal comma",
			source: `--ht-fmt("page.font", "Arial, sans-serif")`,
			want:   `${ page.font | default "Arial, sans-serif" }`,
		},
		{
			name:   "whitespace around fallback",
			source: `--ht-fmt("theme.color"  ,  red  )`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "fallback with pipeline body",
			source: `--ht-fmt("page.title | upper", "DEFAULT")`,
			want:   `${ page.title | upper | default "DEFAULT" }`,
		},
		{
			name:   "CSS length value fallback",
			source: `.a { padding: --ht-fmt("theme.pad", 10px); }`,
			want:   `.a { padding: ${ theme.pad | default "10px" }; }`,
		},
		{
			name:   "explicit empty-string fallback",
			source: `--ht-fmt("theme.color", "")`,
			want:   `${ theme.color | default "" }`,
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
			source: `--ht-fmt("printf \"%s\" page.title")`,
			want:   `${ printf "%s" page.title }`,
		},
		{
			name:   "escaped single quote inside single-quoted",
			source: `--ht-fmt('it\'s')`,
			want:   `${ it's }`,
		},
		{
			name:   "escaped backslash",
			source: `--ht-fmt("a\\b")`,
			want:   `${ a\b }`,
		},
		{
			name:   "unknown escape passes through",
			source: `--ht-fmt("a\nb")`,
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
			source: `my--ht-fmt("x")`,
			want:   `my--ht-fmt("x")`,
		},
		{
			name:   "extra hyphen prefix passes through",
			source: `---ht-fmt("x")`,
			want:   `---ht-fmt("x")`,
		},
		{
			name:   "underscore prefix passes through",
			source: `_--ht-fmt("x")`,
			want:   `_--ht-fmt("x")`,
		},
		{
			name:   "dot prefix is a boundary",
			source: `.--ht-fmt("x")`,
			want:   `.${ httd "x" }`,
		},
		{
			name:   "colon prefix is a boundary",
			source: `color:--ht-fmt("x")`,
			want:   `color:${ httd "x" }`,
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
	source := `--HT-FMT("x"); --Ht-Fmt("y")`
	got, err := rewriteCSS(source)
	if err != nil {
		t.Fatalf("rewriteCSS() error = %v", err)
	}
	if got != source {
		t.Fatalf("rewriteCSS() should leave non-lowercase untouched:\nwant: %q\n got: %q", source, got)
	}
}

func TestRewriteCSS_PassesThroughDollarBrace(t *testing.T) {
	source := `.a { content: "${x}"; color: --ht-fmt("theme.color"); }`
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
			source:   `--ht-fmt("")`,
			contains: "empty template argument",
		},
		{
			name:     "whitespace-only template arg",
			source:   `--ht-fmt("   ")`,
			contains: "empty template argument",
		},
		{
			name:     "bareword template argument",
			source:   `--ht-fmt(x)`,
			contains: "expected quoted argument",
		},
		{
			name:     "keypath without quotes",
			source:   `--ht-fmt(page.title)`,
			contains: "expected quoted argument",
		},
		{
			name:     "unterminated double-quoted string",
			source:   `--ht-fmt("x`,
			contains: "unterminated string",
		},
		{
			name:     "unterminated single-quoted string",
			source:   `--ht-fmt('x`,
			contains: "unterminated string",
		},
		{
			name:     "missing close paren",
			source:   `--ht-fmt("x"`,
			contains: "missing closing )",
		},
		{
			name:     "junk before close paren",
			source:   `--ht-fmt("x" y)`,
			contains: "expected , or )",
		},
		{
			name:     "open paren alone",
			source:   `--ht-fmt(`,
			contains: "end of input",
		},
		{
			name:     "empty fallback",
			source:   `--ht-fmt("x", )`,
			contains: "empty fallback value",
		},
		{
			name:     "trailing comma with nothing after",
			source:   `--ht-fmt("x",`,
			contains: "missing fallback value",
		},
		{
			name:     "unterminated fallback string",
			source:   `--ht-fmt("x", "red`,
			contains: "unterminated string",
		},
		{
			name:     "unterminated unquoted fallback",
			source:   `--ht-fmt("x", rgb(255, 0, 0)`,
			contains: "missing closing )",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rewriteCSS(tc.source)
			if err == nil {
				t.Fatal("rewriteCSS() error = nil, want error")
			}
			if !errors.Is(err, errInvalidHTFmtCall) {
				t.Fatalf("rewriteCSS() error = %v, want errInvalidHTFmtCall", err)
			}
			if !errors.Is(err, errInvalidTemplate) {
				t.Fatalf("rewriteCSS() error should wrap errInvalidTemplate, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("rewriteCSS() error missing substring %q: %v", tc.contains, err)
			}
		})
	}
}

func TestRewriteCSS_InheritsKeypathValidation(t *testing.T) {
	_, err := rewrite(`--ht-fmt("if page.title")`, SyntaxCSS)
	if err == nil {
		t.Fatal("rewrite() error = nil, want invalid-keypath error")
	}
	if !strings.Contains(err.Error(), errInvalidKeypath.Error()) {
		t.Fatalf("rewrite() error = %v, want substring %q", err, errInvalidKeypath.Error())
	}
}

func TestRewriteCSS_InheritsCommentRejection(t *testing.T) {
	_, err := rewrite(`--ht-fmt("/* hidden */")`, SyntaxCSS)
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

	source := `.hero { color: --ht-fmt("theme.color"); }`
	rendered, err := Render(source, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := `.hero { color: red; }`
	if rendered != want {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackWhenNil(t *testing.T) {
	data := testGetter{}

	rendered, err := Render(`.a { color: --ht-fmt("theme.color", red); }`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := `.a { color: red; }`
	if rendered != want {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackWhenEmptyString(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{
			"color": "",
		},
	}

	rendered, err := Render(`.a { color: --ht-fmt("theme.color", red); }`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := `.a { color: red; }`
	if rendered != want {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackSkippedWhenPresent(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{
			"color": "blue",
		},
	}

	rendered, err := Render(`.a { color: --ht-fmt("theme.color", red); }`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := `.a { color: blue; }`
	if rendered != want {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackRGB(t *testing.T) {
	data := testGetter{}

	rendered, err := Render(`.a { color: --ht-fmt("theme.color", rgb(255, 0, 0)); }`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want := `.a { color: rgb(255, 0, 0); }`
	if rendered != want {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", want, rendered)
	}
}

func TestRenderCSS_FallbackResolution(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Page Title",
		},
	}

	rendered, err := Render(`--ht-fmt("site.title, page.title")`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Page Title" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Page Title", rendered)
	}
}

func TestValidateCSS_PropagatesRewriterErrors(t *testing.T) {
	err := validate(`--ht-fmt(x)`, SyntaxCSS)
	if err == nil {
		t.Fatal("validate() error = nil, want error")
	}
	if !errors.Is(err, errInvalidHTFmtCall) {
		t.Fatalf("validate() error should wrap errInvalidHTFmtCall, got %v", err)
	}
}

func TestParseCSS_NormalizedExposesRewritten(t *testing.T) {
	parsed, err := parse(`--ht-fmt("page.title")`, SyntaxCSS)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if parsed.source != `--ht-fmt("page.title")` {
		t.Fatalf("source mismatch:\nwant: %q\n got: %q", `--ht-fmt("page.title")`, parsed.source)
	}

	if parsed.normalized != `${ httd "page.title" }` {
		t.Fatalf("normalized mismatch:\nwant: %q\n got: %q", `${ httd "page.title" }`, parsed.normalized)
	}
}

func TestRewriteCSS_DefaultModeIgnoresCalls(t *testing.T) {
	source := `.a { color: --ht-fmt("theme.color"); }`
	got, err := rewrite(source, SyntaxDefault)
	if err != nil {
		t.Fatalf("rewrite() error = %v", err)
	}
	if got != source {
		t.Fatalf("rewrite() in SyntaxDefault should leave --ht-fmt(...) untouched:\nwant: %q\n got: %q", source, got)
	}
}

func TestHTFallback_InDefaultSyntax(t *testing.T) {
	data := testGetter{}
	rendered, err := Render(`${ page.title | default "Untitled" }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Untitled", rendered)
	}
}

func TestRenderCSS_MissingRestoresCSSSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "double-quoted bare call", source: `--ht-fmt("theme.color")`},
		{name: "single-quoted bare call", source: `--ht-fmt('theme.color')`},
		{name: "inside CSS rule", source: `.a { color: --ht-fmt("theme.color"); }`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := Render(tc.source, testGetter{}, SyntaxCSS)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if rendered != tc.source {
				t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", tc.source, rendered)
			}
		})
	}
}

func TestRenderCSS_DefaultOnMissPreventsRestore(t *testing.T) {
	rendered, err := Render(`--ht-fmt("theme.color", red)`, testGetter{}, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "red" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "red", rendered)
	}
}

func TestRenderCSS_MultiActionAnyMissRestoresWholeCSS(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{"color": "red"},
	}
	source := `.a { color: --ht-fmt("theme.color"); bg: --ht-fmt("theme.bg"); }`

	rendered, err := Render(source, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != source {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", source, rendered)
	}
}
