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
			name:   "pipeline fallback",
			source: `${ site.title, page.title | printf "%v" }`,
			want:   `${ httd "site.title, page.title" | printf "%v" }`,
		},
		{
			name:   "reserved builtin function",
			source: `${ printf "%s" "hello" }`,
			want:   `${ printf "%s" "hello" }`,
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
			source: `${- page.title | printf "%v" -}`,
			want:   `${- httd "page.title" | printf "%v" -}`,
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

	rendered, err := Render("title", `Title: ${ page.title | printf "%s" }`, data, nil, SyntaxDefault)
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
