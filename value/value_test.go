package value

import (
	"fmt"
	"strings"
	"testing"
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

func TestRewrite_DefaultGet(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "bare keypath",
			source: `${ page.title }`,
			want:   `${ get "page.title" }`,
		},
		{
			name:   "string literal fallback",
			source: `${ site.title, page.title, "Untitled" }`,
			want:   `${ get "site.title, page.title" "Untitled" }`,
		},
		{
			name:   "single-quoted string literal fallback",
			source: `${ page.title, 'Untitled' }`,
			want:   `${ get "page.title" "Untitled" }`,
		},
		{
			name:   "reserved get passes through",
			source: `${ get "page.title" }`,
			want:   `${ get "page.title" }`,
		},
		{
			name:   "explicit unquoted get",
			source: `${ get page.title }`,
			want:   `${ get page.title }`,
		},
		{
			name:   "explicit unquoted get fallback",
			source: `${ get site.title, page.title, "Untitled" }`,
			want:   `${ get site.title, page.title, "Untitled" }`,
		},
		{
			name:   "reserved get with single-quoted strings",
			source: `${ get 'page.title' 'Untitled' }`,
			want:   `${ get "page.title" "Untitled" }`,
		},
		{
			name:   "trim markers",
			source: `${- page.title, "X" -}`,
			want:   `${- get "page.title" "X" -}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewrite(tc.source, SyntaxDefault)
			if err != nil {
				t.Fatalf("rewrite() error = %v", err)
			}

			if got != tc.want {
				t.Fatalf("rewrite() mismatch:\nwant: %s\n got: %s", tc.want, got)
			}
		})
	}
}

func TestRewrite_RegisteredFunctionCanStartPipeline(t *testing.T) {
	funcs := FuncMap{
		"upper": strings.ToUpper,
	}

	got, err := rewriteWithFuncs(`${ upper page.title site.title "Placeholder" }`, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("rewriteWithFuncs() error = %v", err)
	}

	want := `${ upper .page.title .site.title "Placeholder" }`
	if got != want {
		t.Fatalf("rewriteWithFuncs() mismatch:\nwant: %s\n got: %s", want, got)
	}
}

func TestRewrite_RegisteredFunctionArguments(t *testing.T) {
	funcs := FuncMap{
		"upper": strings.ToUpper,
		"wrap":  func(args ...any) string { return fmt.Sprint(args...) },
	}

	got, err := rewriteWithFuncs(`${ wrap page.title site 123 true false nil $value .already upper foo-bar (page.title) }`, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("rewriteWithFuncs() error = %v", err)
	}

	want := `${ wrap .page.title .site 123 true false nil $value .already upper foo-bar (page.title) }`
	if got != want {
		t.Fatalf("rewriteWithFuncs() mismatch:\nwant: %s\n got: %s", want, got)
	}
}

func TestValidate_RewritesFunctionArguments(t *testing.T) {
	funcs := FuncMap{
		"upper": strings.ToUpper,
	}

	if err := validateWithFuncs(`${ upper page.title }`, SyntaxDefault, funcs); err != nil {
		t.Fatalf("validateWithFuncs() error = %v", err)
	}
}

func TestValidate_ParentheticalDoesNotImplicitlyGet(t *testing.T) {
	funcs := FuncMap{
		"upper": strings.ToUpper,
	}

	err := validateWithFuncs(`${ upper (page.title) }`, SyntaxDefault, funcs)
	if err == nil {
		t.Fatal("validateWithFuncs() error = nil")
	}
}

func TestValidate_ExplicitGetDoesNotRewriteArguments(t *testing.T) {
	err := validate(`${ get page.title }`, SyntaxDefault)
	if err == nil {
		t.Fatal("validate() error = nil")
	}
	if !strings.Contains(err.Error(), `function "page" not defined`) {
		t.Fatalf("validate() error mismatch:\nwant page function error\n got: %v", err)
	}
}

func TestValidate_RejectsUnsupportedActions(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "comment",
			source: `${ /* hidden */ }`,
			want:   errTemplateComment.Error(),
		},
		{
			name:   "trimmed comment",
			source: `${- /* hidden */ -}`,
			want:   errTemplateComment.Error(),
		},
		{
			name:   "if keyword",
			source: `${ if page.title }`,
			want:   errInvalidKeypath.Error(),
		},
		{
			name:   "range keyword",
			source: `${ range page.items }`,
			want:   errInvalidKeypath.Error(),
		},
		{
			name:   "with keyword",
			source: `${ with page }`,
			want:   errInvalidKeypath.Error(),
		},
		{
			name:   "space-separated tokens",
			source: `${ foo bar }`,
			want:   errInvalidKeypath.Error(),
		},
		{
			name:   "disallowed pipeline function",
			source: `${ page.title | upper }`,
			want:   `not defined`,
		},
		{
			name:   "builtin rejected at head",
			source: `${ printf "%s" "hello" }`,
			want:   errInvalidKeypath.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(tc.source, SyntaxDefault)
			if err == nil {
				t.Fatal("validate() error = nil")
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validate() error mismatch:\nwant substring: %q\n           got: %v", tc.want, err)
			}
		})
	}
}

func TestRender_UsesGetLookup(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Page Title",
		},
	}

	rendered, err := Render(`Title: ${ page.title }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Title: Page Title" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Title: Page Title", rendered)
	}
}

func TestRender_FallsBackAcrossCommaSeparatedKeypaths(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Page Title",
		},
	}

	rendered, err := Render(`${ site.title, page.title }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Page Title" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Page Title", rendered)
	}
}

func TestRender_BareKeywordNameIsValidKeypath(t *testing.T) {
	data := testGetter{
		"template": "Template Value",
	}

	rendered, err := Render(`${ template }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Template Value" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Template Value", rendered)
	}
}

func TestParse_ExposesNormalizedTemplate(t *testing.T) {
	parsed, err := parse(`${ page.title }`, SyntaxDefault)
	if err != nil {
		t.Fatalf("parse() error = %v", err)
	}

	if parsed.source != `${ page.title }` {
		t.Fatalf("source mismatch:\nwant: %q\n got: %q", `${ page.title }`, parsed.source)
	}

	if parsed.normalized != `${ get "page.title" }` {
		t.Fatalf("normalized mismatch:\nwant: %q\n got: %q", `${ get "page.title" }`, parsed.normalized)
	}
}

func TestRender_MissingKeypathRestoresSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "bare keypath", source: `${ page.title }`},
		{name: "with leading text", source: `Title: ${ page.title }`},
		{name: "trim markers", source: `${- page.title -}`},
		{name: "comma fallback all missing", source: `${ site.title, page.title }`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := Render(tc.source, testGetter{}, SyntaxDefault)
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if rendered != tc.source {
				t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", tc.source, rendered)
			}
		})
	}
}

func TestRender_MultiActionAnyMissRestoresWholeSource(t *testing.T) {
	data := testGetter{
		"page": map[string]any{"title": "Hello"},
	}

	source := `${ page.title } and ${ page.missing }`
	rendered, err := Render(source, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != source {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", source, rendered)
	}
}

func TestRender_StringFallbackWinsOnMissPreventsRestore(t *testing.T) {
	rendered, err := Render(`${ page.title, "Untitled" }`, testGetter{}, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Untitled", rendered)
	}
}

func TestRender_StringFallbackWinsOnEmptyString(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": ""}}
	rendered, err := Render(`${ page.title, "Untitled" }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Untitled", rendered)
	}
}

func TestRender_SingleQuotedFallbackWinsOnMiss(t *testing.T) {
	data := testGetter{
		"site": map[string]any{
			"title": "Caleb Hailey",
		},
	}

	rendered, err := Render(`${ site.title } | ${ page.title, '✱' }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Caleb Hailey | ✱" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Caleb Hailey | ✱", rendered)
	}
}

func TestRender_SingleQuotedFallbackDecodesEscapes(t *testing.T) {
	rendered, err := Render(`${ page.title, 'Caleb\'s site' }`, testGetter{}, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Caleb's site" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Caleb's site", rendered)
	}
}

func TestValidate_DefaultFunctionRemoved(t *testing.T) {
	err := validate(`${ page.title | default "Untitled" }`, SyntaxDefault)
	if err == nil {
		t.Fatal("validate() error = nil")
	}
	if !strings.Contains(err.Error(), `function "default" not defined`) {
		t.Fatalf("validate() error mismatch:\nwant default not defined\n got: %v", err)
	}
}

func TestRender_UserFunctionPipeline(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Hello, HyperTemplates",
		},
	}
	funcs := FuncMap{
		"upper": strings.ToUpper,
	}

	rendered, err := Render(`${ page.title | upper }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "HELLO, HYPERTEMPLATES" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "HELLO, HYPERTEMPLATES", rendered)
	}
}

func TestRender_UserFunctionPipelineWithSingleQuotedArgument(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Hello",
		},
	}
	funcs := FuncMap{
		"suffix": func(suffix string, value any) string {
			return fmt.Sprint(value) + suffix
		},
	}

	rendered, err := Render(`${ page.title | suffix '!' }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Hello!" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Hello!", rendered)
	}
}

func TestRender_UserFunctionFirstWithExplicitGet(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Hello",
		},
	}
	funcs := FuncMap{
		"upper": strings.ToUpper,
	}

	rendered, err := Render(`${ upper (get "page.title") }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "HELLO" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "HELLO", rendered)
	}
}

func TestRender_UserFunctionFirstWithDataArguments(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Hello",
		},
		"site": map[string]any{
			"title": "World",
		},
	}
	funcs := FuncMap{
		"join": func(left string, right string) string {
			return left + ", " + right
		},
	}

	rendered, err := Render(`${ join page.title site.title }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Hello, World" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Hello, World", rendered)
	}
}

func TestRender_UserFunctionPipelineArgument(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Hello",
		},
		"site": map[string]any{
			"suffix": "!",
		},
	}
	funcs := FuncMap{
		"suffix": func(suffix string, value any) string {
			return fmt.Sprint(value) + suffix
		},
	}

	rendered, err := Render(`${ page.title | suffix site.suffix }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Hello!" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Hello!", rendered)
	}
}

func TestValidate_UserFunctionPipeline(t *testing.T) {
	funcs := FuncMap{
		"upper": strings.ToUpper,
	}

	if err := validateWithFuncs(`${ page.title | upper }`, SyntaxDefault, funcs); err != nil {
		t.Fatalf("validateWithFuncs() error = %v", err)
	}
}

func TestRender_NoMissNoSubstitution(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": "Page Title"}}
	rendered, err := Render(`Title: ${ page.title }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Title: Page Title" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Title: Page Title", rendered)
	}
}
