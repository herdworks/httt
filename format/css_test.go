package format

import (
	"errors"
	"strings"
	"testing"
	texttemplate "text/template"
)

func TestRewriteCSS_Basic(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "double-quoted keypath",
			source: `--ht-format("theme.color")`,
			want:   `${ httd "theme.color" }`,
		},
		{
			name:   "single-quoted keypath",
			source: `--ht-format('theme.color')`,
			want:   `${ httd "theme.color" }`,
		},
		{
			name:   "fallback keypaths in body",
			source: `--ht-format("site.title, page.title")`,
			want:   `${ httd "site.title, page.title" }`,
		},
		{
			name:   "inside CSS rule",
			source: `.hero { color: --ht-format("theme.color"); }`,
			want:   `.hero { color: ${ httd "theme.color" }; }`,
		},
		{
			name:   "multiple calls in one rule",
			source: `.a { color: --ht-format("theme.color"); bg: --ht-format("theme.bg"); }`,
			want:   `.a { color: ${ httd "theme.color" }; bg: ${ httd "theme.bg" }; }`,
		},
		{
			name:   "whitespace around argument",
			source: `--ht-format(  "x"  )`,
			want:   `${ httd "x" }`,
		},
		{
			name:   "multiline whitespace",
			source: "--ht-format(\n\t\"x\"\n)",
			want:   `${ httd "x" }`,
		},
		{
			name:   "at position 0",
			source: `--ht-format("x")`,
			want:   `${ httd "x" }`,
		},
		{
			name:   "unicode keypath",
			source: `--ht-format("café.title")`,
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
			got, err := Rewrite(tc.source, nil, SyntaxCSS)
			if err != nil {
				t.Fatalf("Rewrite() error = %v", err)
			}

			if got != tc.want {
				t.Fatalf("Rewrite() mismatch:\nwant: %s\n got: %s", tc.want, got)
			}
		})
	}
}

func TestRewriteCSS_Pipeline(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
		funcs  texttemplate.FuncMap
	}{
		{
			name:   "pipeline tail with text/template printf",
			source: `--ht-format("page.title | printf \"[%s]\"")`,
			want:   `${ httd "page.title" | printf "[%s]" }`,
		},
		{
			name:   "first-token non-reserved gets httd-wrapped",
			source: `--ht-format('printf "%s" page.title')`,
			want:   `${ httd "printf \"%s\" page.title" }`,
		},
		{
			name:   "pipeline with user func",
			source: `--ht-format("page.title | upper")`,
			want:   `${ httd "page.title" | upper }`,
			funcs: texttemplate.FuncMap{
				"upper": strings.ToUpper,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Rewrite(tc.source, tc.funcs, SyntaxCSS)
			if err != nil {
				t.Fatalf("Rewrite() error = %v", err)
			}

			if got != tc.want {
				t.Fatalf("Rewrite() mismatch:\nwant: %s\n got: %s", tc.want, got)
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
			source: `--ht-format("theme.color", red)`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "quoted fallback",
			source: `--ht-format("theme.color", "red")`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "single-quoted fallback",
			source: `--ht-format("theme.color", 'red')`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "fallback with internal commas and parens",
			source: `--ht-format("theme.color", rgb(255, 0, 0))`,
			want:   `${ theme.color | default "rgb(255, 0, 0)" }`,
		},
		{
			name:   "quoted fallback with internal comma",
			source: `--ht-format("page.font", "Arial, sans-serif")`,
			want:   `${ page.font | default "Arial, sans-serif" }`,
		},
		{
			name:   "whitespace around fallback",
			source: `--ht-format("theme.color"  ,  red  )`,
			want:   `${ theme.color | default "red" }`,
		},
		{
			name:   "fallback with pipeline body",
			source: `--ht-format("page.title | upper", "DEFAULT")`,
			want:   `${ page.title | upper | default "DEFAULT" }`,
		},
		{
			name:   "CSS length value fallback",
			source: `.a { padding: --ht-format("theme.pad", 10px); }`,
			want:   `.a { padding: ${ theme.pad | default "10px" }; }`,
		},
		{
			name:   "explicit empty-string fallback",
			source: `--ht-format("theme.color", "")`,
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
			source: `--ht-format("printf \"%s\" page.title")`,
			want:   `${ printf "%s" page.title }`,
		},
		{
			name:   "escaped single quote inside single-quoted",
			source: `--ht-format('it\'s')`,
			want:   `${ it's }`,
		},
		{
			name:   "escaped backslash",
			source: `--ht-format("a\\b")`,
			want:   `${ a\b }`,
		},
		{
			name:   "unknown escape passes through",
			source: `--ht-format("a\nb")`,
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
			source: `my--ht-format("x")`,
			want:   `my--ht-format("x")`,
		},
		{
			name:   "extra hyphen prefix passes through",
			source: `---ht-format("x")`,
			want:   `---ht-format("x")`,
		},
		{
			name:   "underscore prefix passes through",
			source: `_--ht-format("x")`,
			want:   `_--ht-format("x")`,
		},
		{
			name:   "dot prefix is a boundary",
			source: `.--ht-format("x")`,
			want:   `.${ httd "x" }`,
		},
		{
			name:   "colon prefix is a boundary",
			source: `color:--ht-format("x")`,
			want:   `color:${ httd "x" }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Rewrite(tc.source, nil, SyntaxCSS)
			if err != nil {
				t.Fatalf("Rewrite() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("Rewrite() mismatch:\nwant: %q\n got: %q", tc.want, got)
			}
		})
	}
}

func TestRewriteCSS_CaseSensitive(t *testing.T) {
	source := `--HT-FORMAT("x"); --Ht-Format("y")`
	got, err := rewriteCSS(source)
	if err != nil {
		t.Fatalf("rewriteCSS() error = %v", err)
	}
	if got != source {
		t.Fatalf("rewriteCSS() should leave non-lowercase untouched:\nwant: %q\n got: %q", source, got)
	}
}

func TestRewriteCSS_PassesThroughDollarBrace(t *testing.T) {
	source := `.a { content: "${x}"; color: --ht-format("theme.color"); }`
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
			source:   `--ht-format("")`,
			contains: "empty template argument",
		},
		{
			name:     "whitespace-only template arg",
			source:   `--ht-format("   ")`,
			contains: "empty template argument",
		},
		{
			name:     "bareword template argument",
			source:   `--ht-format(x)`,
			contains: "expected quoted argument",
		},
		{
			name:     "keypath without quotes",
			source:   `--ht-format(page.title)`,
			contains: "expected quoted argument",
		},
		{
			name:     "unterminated double-quoted string",
			source:   `--ht-format("x`,
			contains: "unterminated string",
		},
		{
			name:     "unterminated single-quoted string",
			source:   `--ht-format('x`,
			contains: "unterminated string",
		},
		{
			name:     "missing close paren",
			source:   `--ht-format("x"`,
			contains: "missing closing )",
		},
		{
			name:     "junk before close paren",
			source:   `--ht-format("x" y)`,
			contains: "expected , or )",
		},
		{
			name:     "open paren alone",
			source:   `--ht-format(`,
			contains: "end of input",
		},
		{
			name:     "empty fallback",
			source:   `--ht-format("x", )`,
			contains: "empty fallback value",
		},
		{
			name:     "trailing comma with nothing after",
			source:   `--ht-format("x",`,
			contains: "missing fallback value",
		},
		{
			name:     "unterminated fallback string",
			source:   `--ht-format("x", "red`,
			contains: "unterminated string",
		},
		{
			name:     "unterminated unquoted fallback",
			source:   `--ht-format("x", rgb(255, 0, 0)`,
			contains: "missing closing )",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rewriteCSS(tc.source)
			if err == nil {
				t.Fatal("rewriteCSS() error = nil, want error")
			}
			if !errors.Is(err, ErrInvalidHTFormatCall) {
				t.Fatalf("rewriteCSS() error = %v, want ErrInvalidHTFormatCall", err)
			}
			if !errors.Is(err, ErrInvalidTemplate) {
				t.Fatalf("rewriteCSS() error should wrap ErrInvalidTemplate, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("rewriteCSS() error missing substring %q: %v", tc.contains, err)
			}
		})
	}
}

func TestRewriteCSS_InheritsReservedKeywords(t *testing.T) {
	_, err := Rewrite(`--ht-format("if page.title")`, nil, SyntaxCSS)
	if err == nil {
		t.Fatal("Rewrite() error = nil, want reserved-action error")
	}
	if !strings.Contains(err.Error(), ErrReservedAction.Error()) {
		t.Fatalf("Rewrite() error = %v, want substring %q", err, ErrReservedAction.Error())
	}
}

func TestRewriteCSS_InheritsCommentRejection(t *testing.T) {
	_, err := Rewrite(`--ht-format("/* hidden */")`, nil, SyntaxCSS)
	if err == nil {
		t.Fatal("Rewrite() error = nil, want template-comment error")
	}
	if !strings.Contains(err.Error(), ErrTemplateComment.Error()) {
		t.Fatalf("Rewrite() error = %v, want substring %q", err, ErrTemplateComment.Error())
	}
}

func TestRenderCSS_EndToEnd(t *testing.T) {
	data := testGetter{
		"theme": map[string]any{
			"color": "red",
		},
	}

	source := `.hero { color: --ht-format("theme.color"); }`
	rendered, err := Render("css", source, data, nil, SyntaxCSS)
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

	rendered, err := Render("css", `.a { color: --ht-format("theme.color", red); }`, data, nil, SyntaxCSS)
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

	rendered, err := Render("css", `.a { color: --ht-format("theme.color", red); }`, data, nil, SyntaxCSS)
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

	rendered, err := Render("css", `.a { color: --ht-format("theme.color", red); }`, data, nil, SyntaxCSS)
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

	rendered, err := Render("css", `.a { color: --ht-format("theme.color", rgb(255, 0, 0)); }`, data, nil, SyntaxCSS)
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

	rendered, err := Render("css", `--ht-format("site.title, page.title")`, data, nil, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Page Title" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Page Title", rendered)
	}
}

func TestRenderCSS_PipelineWithBuiltin(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Page Title",
		},
	}

	rendered, err := Render("css", `--ht-format("page.title | printf \"[%s]\"")`, data, nil, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "[Page Title]" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "[Page Title]", rendered)
	}
}

func TestValidateCSS_PropagatesRewriterErrors(t *testing.T) {
	err := Validate(`--ht-format(x)`, nil, SyntaxCSS)
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !errors.Is(err, ErrInvalidHTFormatCall) {
		t.Fatalf("Validate() error should wrap ErrInvalidHTFormatCall, got %v", err)
	}
}

func TestParseCSS_NormalizedExposesRewritten(t *testing.T) {
	tmpl, err := Parse("css", `--ht-format("page.title")`, nil, SyntaxCSS)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if tmpl.Source() != `--ht-format("page.title")` {
		t.Fatalf("Source() mismatch:\nwant: %q\n got: %q", `--ht-format("page.title")`, tmpl.Source())
	}

	if tmpl.Normalized() != `${ httd "page.title" }` {
		t.Fatalf("Normalized() mismatch:\nwant: %q\n got: %q", `${ httd "page.title" }`, tmpl.Normalized())
	}
}

func TestRewriteCSS_DefaultModeIgnoresCalls(t *testing.T) {
	source := `.a { color: --ht-format("theme.color"); }`
	got, err := Rewrite(source, nil, SyntaxDefault)
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if got != source {
		t.Fatalf("Rewrite() in SyntaxDefault should leave --ht-format(...) untouched:\nwant: %q\n got: %q", source, got)
	}
}

func TestHTFallback_InDefaultSyntax(t *testing.T) {
	data := testGetter{}
	rendered, err := Render("default", `${ page.title | default "Untitled" }`, data, nil, SyntaxDefault)
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
		{name: "double-quoted bare call", source: `--ht-format("theme.color")`},
		{name: "single-quoted bare call", source: `--ht-format('theme.color')`},
		{name: "inside CSS rule", source: `.a { color: --ht-format("theme.color"); }`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := Render("css", tc.source, testGetter{}, nil, SyntaxCSS)
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
	rendered, err := Render("css", `--ht-format("theme.color", red)`, testGetter{}, nil, SyntaxCSS)
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
	source := `.a { color: --ht-format("theme.color"); bg: --ht-format("theme.bg"); }`

	rendered, err := Render("css", source, data, nil, SyntaxCSS)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != source {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", source, rendered)
	}
}
