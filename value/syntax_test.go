package value

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
			source: `--ht-value("theme.color")`,
			want:   `${ httd "theme.color" }`,
		},
		{
			name:   "single-quoted keypath",
			source: `--ht-value('theme.color')`,
			want:   `${ httd "theme.color" }`,
		},
		{
			name:   "fallback keypaths in body",
			source: `--ht-value("site.title, page.title")`,
			want:   `${ httd "site.title, page.title" }`,
		},
		{
			name:   "inside CSS rule",
			source: `.hero { color: --ht-value("theme.color"); }`,
			want:   `.hero { color: ${ httd "theme.color" }; }`,
		},
		{
			name:   "multiple calls in one rule",
			source: `.a { color: --ht-value("theme.color"); bg: --ht-value("theme.bg"); }`,
			want:   `.a { color: ${ httd "theme.color" }; bg: ${ httd "theme.bg" }; }`,
		},
		{
			name:   "whitespace around argument",
			source: `--ht-value(  "x"  )`,
			want:   `${ httd "x" }`,
		},
		{
			name:   "multiline whitespace",
			source: "--ht-value(\n\t\"x\"\n)",
			want:   `${ httd "x" }`,
		},
		{
			name:   "at position 0",
			source: `--ht-value("x")`,
			want:   `${ httd "x" }`,
		},
		{
			name:   "unicode keypath",
			source: `--ht-value("café.title")`,
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
			source: `--ht-value("page.title | default \"Untitled\"")`,
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
			source: `--ht-value("theme.color", red)`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "quoted fallback",
			source: `--ht-value("theme.color", "red")`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "single-quoted fallback",
			source: `--ht-value("theme.color", 'red')`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "fallback with internal commas and parens",
			source: `--ht-value("theme.color", rgb(255, 0, 0))`,
			want:   `${ theme.color | default "rgb(255, 0, 0)" }`,
		},
		{
			name:   "quoted fallback with internal comma",
			source: `--ht-value("page.font", "Arial, sans-serif")`,
			want:   `${ page.font | default "Arial, sans-serif" }`,
		},
		{
			name:   "whitespace around fallback",
			source: `--ht-value("theme.color"  ,  red  )`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "fallback with pipeline body",
			source: `--ht-value("page.title | upper", "DEFAULT")`,
			want:   `${ page.title | upper | default "DEFAULT" }`,
		},
		{
			name:   "CSS length value fallback",
			source: `.a { padding: --ht-value("theme.pad", 10px); }`,
			want:   `.a { padding: ${ theme.pad | default "10px" }; }`,
		},
		{
			name:   "explicit empty-string fallback",
			source: `--ht-value("theme.color", "")`,
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
			want:   `.${ httd "x" }`,
		},
		{
			name:   "colon prefix is a boundary",
			source: `color:--ht-value("x")`,
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
		t.Fatal("rewrite() error = nil, want invalid-keypath error")
	}
	if !strings.Contains(err.Error(), errInvalidKeypath.Error()) {
		t.Fatalf("rewrite() error = %v, want substring %q", err, errInvalidKeypath.Error())
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

	rendered, err := Render(`.a { color: --ht-value("theme.color", red); }`, data, SyntaxCSS)
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

	rendered, err := Render(`.a { color: --ht-value("theme.color", red); }`, data, SyntaxCSS)
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

	rendered, err := Render(`.a { color: --ht-value("theme.color", red); }`, data, SyntaxCSS)
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

	rendered, err := Render(`.a { color: --ht-value("theme.color", rgb(255, 0, 0)); }`, data, SyntaxCSS)
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

	rendered, err := Render(`--ht-value("site.title, page.title")`, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Page Title" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Page Title", rendered)
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

	if parsed.normalized != `${ httd "page.title" }` {
		t.Fatalf("normalized mismatch:\nwant: %q\n got: %q", `${ httd "page.title" }`, parsed.normalized)
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
		{name: "double-quoted bare call", source: `--ht-value("theme.color")`},
		{name: "single-quoted bare call", source: `--ht-value('theme.color')`},
		{name: "inside CSS rule", source: `.a { color: --ht-value("theme.color"); }`},
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
	rendered, err := Render(`--ht-value("theme.color", red)`, testGetter{}, SyntaxCSS)
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
	source := `.a { color: --ht-value("theme.color"); bg: --ht-value("theme.bg"); }`

	rendered, err := Render(source, data, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != source {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", source, rendered)
	}
}
