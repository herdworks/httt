package variable

import (
	"errors"
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
			want:   `function "if" not registered`,
		},
		{
			name:   "range keyword",
			source: `${ range page.items }`,
			want:   `function "range" not registered`,
		},
		{
			name:   "with keyword",
			source: `${ with page }`,
			want:   `function "with" not registered`,
		},
		{
			name:   "space-separated tokens",
			source: `${ foo bar }`,
			want:   `function "foo" not registered`,
		},
		{
			name:   "disallowed pipeline function",
			source: `${ page.title | upper }`,
			want:   `function "upper" not registered`,
		},
		{
			name:   "builtin rejected at head",
			source: `${ printf "%s" "hello" }`,
			want:   `function "printf" not registered`,
		},
		{
			name:   "builtin rejected at pipeline tail",
			source: `${ page.title | printf "%v" }`,
			want:   `function "printf" not registered`,
		},
		{
			name:   "builtin len rejected at pipeline tail",
			source: `${ page.title | len }`,
			want:   `function "len" not registered`,
		},
		{
			name:   "builtin index rejected at pipeline tail",
			source: `${ page.items | index 0 }`,
			want:   `function "index" not registered`,
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

	rendered, err := RenderString(`Title: ${ page.title }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "Title: Page Title" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Title: Page Title", rendered)
	}
}

func TestRender_FallsBackAcrossCommaSeparatedKeypaths(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Page Title",
		},
	}

	rendered, err := RenderString(`${ site.title, page.title }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "Page Title" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Page Title", rendered)
	}
}

func TestRender_BareKeywordNameIsValidKeypath(t *testing.T) {
	data := testGetter{
		"template": "Template Value",
	}

	rendered, err := RenderString(`${ template }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "Template Value" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Template Value", rendered)
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

func TestRender_MissingKeypathErrors(t *testing.T) {
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
			_, err := RenderString(tc.source, testGetter{}, SyntaxDefault)
			if err == nil {
				t.Fatal("RenderString() error = nil, want MissingValueError")
			}
			var miss *MissingValueError
			if !errors.As(err, &miss) {
				t.Fatalf("RenderString() error = %v, want MissingValueError", err)
			}
			if miss.Source != tc.source {
				t.Fatalf("MissingValueError.Source = %q, want %q", miss.Source, tc.source)
			}
		})
	}
}

func TestRender_MultiActionAnyMissErrors(t *testing.T) {
	data := testGetter{
		"page": map[string]any{"title": "Hello"},
	}

	source := `${ page.title } and ${ page.missing }`
	_, err := RenderString(source, data, SyntaxDefault)
	if err == nil {
		t.Fatal("RenderString() error = nil, want MissingValueError")
	}
	var miss *MissingValueError
	if !errors.As(err, &miss) {
		t.Fatalf("RenderString() error = %v, want MissingValueError", err)
	}
	if miss.Source != source {
		t.Fatalf("MissingValueError.Source = %q, want %q", miss.Source, source)
	}
}

func TestRender_NullValueErrors(t *testing.T) {
	data := testGetter{
		"page": map[string]any{"title": nil},
	}
	_, err := RenderString(`${ page.title }`, data, SyntaxDefault)
	if err == nil {
		t.Fatal("RenderString() error = nil, want MissingValueError")
	}
	var miss *MissingValueError
	if !errors.As(err, &miss) {
		t.Fatalf("RenderString() error = %v, want MissingValueError", err)
	}
}

func TestRender_NullDescendantErrors(t *testing.T) {
	data := testGetter{
		"page": map[string]any{"foo": nil},
	}
	_, err := RenderString(`${ page.foo.bar }`, data, SyntaxDefault)
	if err == nil {
		t.Fatal("RenderString() error = nil, want MissingValueError")
	}
	var miss *MissingValueError
	if !errors.As(err, &miss) {
		t.Fatalf("RenderString() error = %v, want MissingValueError", err)
	}
}

func TestRender_EmptyStringIsValidNoError(t *testing.T) {
	data := testGetter{
		"page": map[string]any{"title": ""},
	}
	rendered, err := RenderString(`${ page.title }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v, want no error for empty string value", err)
	}
	if rendered != "" {
		t.Fatalf("RenderString() = %q, want empty string", rendered)
	}
}

func TestRender_NullValueWithFallbackUsesFallback(t *testing.T) {
	data := testGetter{
		"page": map[string]any{"title": nil},
	}
	rendered, err := RenderString(`${ page.title, "Untitled" }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("RenderString() = %q, want %q", rendered, "Untitled")
	}
}

func TestRender_StringFallbackWinsOnMissPreventsRestore(t *testing.T) {
	rendered, err := RenderString(`${ page.title, "Untitled" }`, testGetter{}, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Untitled", rendered)
	}
}

func TestRender_StringFallbackWinsOnEmptyString(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": ""}}
	rendered, err := RenderString(`${ page.title, "Untitled" }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Untitled", rendered)
	}
}

func TestRender_SingleQuotedFallbackWinsOnMiss(t *testing.T) {
	data := testGetter{
		"site": map[string]any{
			"title": "Caleb Hailey",
		},
	}

	rendered, err := RenderString(`${ site.title } | ${ page.title, '✱' }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "Caleb Hailey | ✱" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Caleb Hailey | ✱", rendered)
	}
}

func TestRender_SingleQuotedFallbackDecodesEscapes(t *testing.T) {
	rendered, err := RenderString(`${ page.title, 'Caleb\'s site' }`, testGetter{}, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "Caleb's site" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Caleb's site", rendered)
	}
}

func TestValidate_DefaultFunctionRemoved(t *testing.T) {
	err := validate(`${ page.title | default "Untitled" }`, SyntaxDefault)
	if err == nil {
		t.Fatal("validate() error = nil")
	}
	if !strings.Contains(err.Error(), `function "default" not registered`) {
		t.Fatalf("validate() error mismatch:\nwant default not registered\n got: %v", err)
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

	rendered, err := RenderString(`${ page.title | upper }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "HELLO, HYPERTEMPLATES" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "HELLO, HYPERTEMPLATES", rendered)
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

	rendered, err := RenderString(`${ page.title | suffix '!' }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "Hello!" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Hello!", rendered)
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

	rendered, err := RenderString(`${ upper (get "page.title") }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "HELLO" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "HELLO", rendered)
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

	rendered, err := RenderString(`${ join page.title site.title }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "Hello, World" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Hello, World", rendered)
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

	rendered, err := RenderString(`${ page.title | suffix site.suffix }`, data, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}

	if rendered != "Hello!" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Hello!", rendered)
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
	rendered, err := RenderString(`Title: ${ page.title }`, data, SyntaxDefault)
	if err != nil {
		t.Fatalf("RenderString() error = %v", err)
	}
	if rendered != "Title: Page Title" {
		t.Fatalf("RenderString() mismatch:\nwant: %q\n got: %q", "Title: Page Title", rendered)
	}
}
