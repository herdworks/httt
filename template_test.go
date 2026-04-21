package hypertemplates

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"testing"

	"golang.org/x/net/html"
)

// Template.ResolveIncludes
// Ensure that ht-include elements are properly resolved.
func TestResolveIncludes(t *testing.T) {
	tests := []struct {
		name   string
		layout string
		theme  string
		err    error
	}{
		{
			name:   "resolve includes",
			layout: "testdata/ht-include/layouts/ok.html",
			theme:  "testdata/ht-include/theme.json",
			err:    nil,
		},
		{
			name:   "broken include",
			layout: "testdata/ht-include/layouts/broken.html",
			theme:  "testdata/ht-include/theme.json",
			err:    ErrIncludeFragment,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var layoutBytes []byte
			var layoutHTML *html.Node
			var theme *Theme = &Theme{URI: &url.URL{Path: tc.theme}}
			var err error
			if layoutBytes, err = os.ReadFile(tc.layout); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("unexpected error reading layout %q bytes:\n\twanted: %v\n\t   got: %v", tc.layout, tc.err, err)
			}
			if layoutHTML, err = html.Parse(bytes.NewReader(layoutBytes)); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("failed parsing layout %q html: %v", tc.layout, err)
			}
			template := Template{Layout: layoutHTML, Theme: theme}
			if template.Layout == nil {
				t.Fatalf("template.Layout is nil")
			}
			if err = template.ResolveIncludes(); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("error resolving includes: %v", err)
			}
			if tc.err != nil {
				t.Fatalf("expected error %q but got none", tc.err)
			}
			// t.Logf("\n%s\n", template.HTML(template.Layout))
		})
	}
}

// TestAttributeForwarding
// Ensure that attributes are forwarded from ht-include placeholder elements to an included fragment.
func TestAttributeForwarding(t *testing.T) {
	tests := []struct {
		name   string
		layout string
		theme  string
	}{
		{
			name:   "forward attributes",
			theme:  "testdata/ht-include/theme.json",
			layout: "testdata/ht-include/layouts/ok.html",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var theme *Theme = &Theme{URI: &url.URL{Path: tc.theme}}
			var layoutBytes []byte
			var layoutHTML *html.Node
			var err error
			if layoutBytes, err = os.ReadFile(tc.layout); err != nil {
				t.Fatalf("error reading layout %q bytes: %v", tc.layout, err)
			}
			if layoutHTML, err = html.Parse(bytes.NewReader(layoutBytes)); err != nil {
				t.Fatalf("error parsing layout %q html: %v", tc.layout, err)
			}
			template := Template{Layout: layoutHTML, Theme: theme}
			if template.Layout == nil {
				t.Fatalf("template.Layout is nil")
			}
			if err = template.ResolveIncludes(); err != nil {
				t.Fatalf("error resolving includes: %v", err)
			}
			if matches := QueryAll(template.Layout, "header#hero"); len(matches) != 1 {
				t.Fatalf("expected 1 match for header#hero, got %d", len(matches))
			}
			if matches := QueryAll(template.Layout, "footer#forwarded"); len(matches) != 1 {
				t.Fatalf("expected 1 match for footer#forwarded, got %d", len(matches))
			}
		})
	}
}

// TestConditionalParser
// TODO: Ensure that ht-if attributes are properly parsed.

// TestConditionalExists
// TODO: Ensure that ht-if attribute's with no conditions check for existence.
// <element ht-if="some.key.path"></element> should resolve true if { some: { key: { path: { x: null } } } } (i.e. even if path is a map with at least one key).

// TestConditionalEvaluation
// TODO: Ensure that ht-if attributes are properly compared against TemplateData values.

// Template.RenderBlocks
// Ensure that ht-block elements are properly rendered.
func TestRenderBlocks(t *testing.T) {
	tests := []struct {
		name   string
		theme  string
		layout string
		data   string
		err    error
	}{
		{
			name:   "resolve blocks",
			theme:  "testdata/ht-block/theme.json",
			layout: "layouts/ok.html",
			data:   "testdata/ht-block/content/index.md",
			err:    nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var theme *Theme
			var layout *html.Node
			var data TemplateData
			var template *Template
			var htmlBytes []byte
			var err error

			// configure the theme
			theme = &Theme{URI: &url.URL{Path: tc.theme}}
			theme.ResolveConfig()

			// load the layout
			if layout, err = theme.ResourceHTML(tc.layout); err != nil {
				t.Fatalf("error loading layout %q: %v", tc.layout, err)
			}

			// initialize the template
			template = &Template{
				Theme:          theme,
				Layout:         layout,
				TidyMode:       true,
				MarkdownConfig: nil,
			}

			// parse the template data
			if data, err = ParseTemplateDataFile(tc.data, nil); err != nil {
				t.Fatalf("error parsing template data file %q: %v", tc.data, err)
			}

			// render the template
			if htmlBytes, err = template.Process(&data, nil); err != nil {
				t.Fatalf("error processing template: %v", err)
			}

			// print the rendered HTML for debugging
			t.Logf("\n%s\n", string(htmlBytes))
		})
	}
}

// TestRenderBlockPageFallback reproduces the caller setup where the outer page
// data is passed as userData under a "page" namespace, and a block template
// references page.* paths via the ht-attrs / ht-content fallback chain.
func TestRenderBlockPageFallback(t *testing.T) {
	theme := &Theme{URI: &url.URL{Path: "testdata/ht-block/theme.json"}}
	theme.ResolveConfig()

	layout, err := theme.ResourceHTML("layouts/ok.html")
	if err != nil {
		t.Fatalf("error loading layout: %v", err)
	}

	pageData, err := ParseTemplateDataFile("testdata/ht-block/content/attachment.md", nil)
	if err != nil {
		t.Fatalf("error parsing page data: %v", err)
	}

	// Mirror hypertexting-go's Attachments.TemplateData() which returns
	// the named type TemplateDataSet (not []TemplateData). Replace the
	// YAML-parsed slice so we reproduce the real caller's shape.
	pageData["attachments"] = TemplateDataSet{
		TemplateData{
			"kind": "link",
			"href": "https://www.anker.com/products/a2345-anker-prime-charger-250w-6-ports-ganprime",
			"data": TemplateData{
				"hypergraph": TemplateData{
					"href":        "https://www.anker.com/products/a2345-anker-prime-charger-250w-6-ports-ganprime",
					"cover":       "https://example.com/cover.png",
					"favicon":     "https://www.anker.com/favicon.png",
					"title":       "Anker Prime Charger (250W, 6 Ports, GaNPrime) - Anker US",
					"description": "Power up with the Anker Prime Desktop Charger.",
				},
			},
		},
	}

	template := &Template{
		Theme:          theme,
		Layout:         layout,
		TidyMode:       true,
		MarkdownConfig: nil,
	}

	// Mirror the caller: outer data may or may not have page.*, but userData
	// wraps the page data under the "page" key explicitly.
	userData := &TemplateData{"page": pageData}

	htmlBytes, err := template.Process(&pageData, userData)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}
	t.Logf("\n%s\n", string(htmlBytes))

	if !bytes.Contains(htmlBytes, []byte("anker.com/products/a2345")) {
		t.Errorf("expected rendered output to include page.attachments.1.data.hypergraph.href; got:\n%s", htmlBytes)
	}
}
