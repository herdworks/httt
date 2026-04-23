package format

import (
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

func TestRewrite_DefaultHTTD(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "bare keypath",
			source: `${ page.title }`,
			want:   `${ httd "page.title" }`,
		},
		{
			name:   "pipeline with default",
			source: `${ site.title, page.title | default "Untitled" }`,
			want:   `${ httd "site.title, page.title" | default "Untitled" }`,
		},
		{
			name:   "reserved httd passes through",
			source: `${ httd "page.title" }`,
			want:   `${ httd "page.title" }`,
		},
		{
			name:   "trim markers",
			source: `${- page.title | default "X" -}`,
			want:   `${- httd "page.title" | default "X" -}`,
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

func TestRender_UsesHTTDLookup(t *testing.T) {
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

	if parsed.normalized != `${ httd "page.title" }` {
		t.Fatalf("normalized mismatch:\nwant: %q\n got: %q", `${ httd "page.title" }`, parsed.normalized)
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

func TestRender_DefaultWinsOnMissPreventsRestore(t *testing.T) {
	rendered, err := Render(`${ page.title | default "Untitled" }`, testGetter{}, SyntaxDefault)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if rendered != "Untitled" {
		t.Fatalf("Render() mismatch:\nwant: %q\n got: %q", "Untitled", rendered)
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
