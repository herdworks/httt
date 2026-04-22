package format

import (
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

func TestRewrite_DefaultHTTD(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
		funcs  texttemplate.FuncMap
	}{
		{
			name:   "bare keypath",
			source: `${ page.title }`,
			want:   `${ httd "page.title" }`,
		},
		{
			name:   "pipeline with user func",
			source: `${ site.title, page.title | upper }`,
			want:   `${ httd "site.title, page.title" | upper }`,
			funcs: texttemplate.FuncMap{
				"upper": strings.ToUpper,
			},
		},
		{
			name:   "reserved httd passes through",
			source: `${ httd "page.title" }`,
			want:   `${ httd "page.title" }`,
		},
		{
			name:   "reserved custom function",
			source: `${ upper "hello" }`,
			want:   `${ upper "hello" }`,
			funcs: texttemplate.FuncMap{
				"upper": strings.ToUpper,
			},
		},
		{
			name:   "trim markers",
			source: `${- page.title | upper -}`,
			want:   `${- httd "page.title" | upper -}`,
			funcs: texttemplate.FuncMap{
				"upper": strings.ToUpper,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Rewrite(tc.source, tc.funcs, SyntaxDefault)
			if err != nil {
				t.Fatalf("Rewrite() error = %v", err)
			}

			if got != tc.want {
				t.Fatalf("Rewrite() mismatch:\nwant: %s\n got: %s", tc.want, got)
			}
		})
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
			want:   ErrTemplateComment.Error(),
		},
		{
			name:   "trimmed comment",
			source: `${- /* hidden */ -}`,
			want:   ErrTemplateComment.Error(),
		},
		{
			name:   "if keyword",
			source: `${ if page.title }`,
			want:   ErrReservedAction.Error(),
		},
		{
			name:   "range keyword",
			source: `${ range page.items }`,
			want:   ErrReservedAction.Error(),
		},
		{
			name:   "with keyword",
			source: `${ with page }`,
			want:   ErrReservedAction.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.source, nil, SyntaxDefault)
			if err == nil {
				t.Fatal("Validate() error = nil")
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate() error mismatch:\nwant substring: %q\n           got: %v", tc.want, err)
			}
		})
	}
}

func TestRender_UsesHTTDLookup(t *testing.T) {
	data := testGetter{
		"page": map[string]any{
			"title": "Page Title",
		},
	}

	rendered, err := Render("title", `Title: ${ page.title }`, data, nil, SyntaxDefault)
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

	rendered, err := Render("title", `${ site.title, page.title }`, data, nil, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if rendered != "Page Title" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Page Title", rendered)
	}
}

func TestParse_ExposesNormalizedTemplate(t *testing.T) {
	tmpl, err := Parse("title", `${ page.title }`, nil, SyntaxDefault)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if tmpl.Source() != `${ page.title }` {
		t.Fatalf("Source() mismatch:\nwant: %q\n got: %q", `${ page.title }`, tmpl.Source())
	}

	if tmpl.Normalized() != `${ httd "page.title" }` {
		t.Fatalf("Normalized() mismatch:\nwant: %q\n got: %q", `${ httd "page.title" }`, tmpl.Normalized())
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
			rendered, err := Render("t", tc.source, testGetter{}, nil, SyntaxDefault)
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
	rendered, err := Render("t", source, data, nil, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != source {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", source, rendered)
	}
}

func TestRender_DefaultWinsOnMissPreventsRestore(t *testing.T) {
	rendered, err := Render("t", `${ page.title | default "Untitled" }`, testGetter{}, nil, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Untitled", rendered)
	}
}

func TestRender_NoMissNoSubstitution(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": "Page Title"}}
	rendered, err := Render("t", `Title: ${ page.title }`, data, nil, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Title: Page Title" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Title: Page Title", rendered)
	}
}

func TestRender_DroppedBuiltinsFallThrough(t *testing.T) {
	source := `${ printf "%s" "hello" }`
	rendered, err := Render("t", source, testGetter{}, nil, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != source {
		t.Fatalf("Render() mismatch (dropped printf builtin should surface as source):\nwant: %q\n got: %q", source, rendered)
	}
}

func TestIsReservedFunction_NoBuiltins(t *testing.T) {
	for _, name := range []string{"printf", "print", "and", "or", "len", "index", "eq", "ne", "lt", "gt", "html", "js", "urlquery", "slice"} {
		if isReservedFunction(name, nil) {
			t.Fatalf("isReservedFunction(%q) = true; want false (builtins dropped)", name)
		}
	}
	if !isReservedFunction("httd", nil) {
		t.Fatal("isReservedFunction(\"httd\") = false; want true")
	}
	if !isReservedFunction("default", nil) {
		t.Fatal("isReservedFunction(\"default\") = false; want true")
	}
	if !isReservedFunction("upper", texttemplate.FuncMap{"upper": strings.ToUpper}) {
		t.Fatal("isReservedFunction(\"upper\") with registered func = false; want true")
	}
}
