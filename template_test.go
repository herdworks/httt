package hypertemplates

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/herdworks/hypertemplates-go/scripting"
	htvalue "github.com/herdworks/hypertemplates-go/value"
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

func TestRenderValues(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<section ht-apply data-href="${ page.href }">
					Hello ${ page.title }
					<span>${ page.slug }</span>
				</section>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"title": "Hello, HyperTemplates",
			"href":  "https://example.com/posts/hello",
			"slug":  "hello-hypertemplates",
		},
	}

	template := &Template{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := template.Process(&data, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	renderedLayout, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	section := Query(renderedLayout, "section")
	if section == nil {
		t.Fatalf("expected rendered output to include section; got:\n%s", htmlBytes)
	}

	if href := getAttributeString(section, "data-href"); href != "https://example.com/posts/hello" {
		t.Fatalf("expected rendered data-href %q, got %q", "https://example.com/posts/hello", href)
	}

	var directText string
	for child := section.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.TextNode {
			continue
		}
		if !strings.Contains(child.Data, "Hello, HyperTemplates") {
			continue
		}
		directText = strings.TrimSpace(child.Data)
		break
	}

	if directText != "Hello Hello, HyperTemplates" {
		t.Fatalf("expected rendered direct text %q, got %q", "Hello Hello, HyperTemplates", directText)
	}

	span := Query(renderedLayout, "section span")
	if span == nil {
		t.Fatalf("expected rendered output to include nested span; got:\n%s", htmlBytes)
	}

	if text := strings.TrimSpace(span.FirstChild.Data); text != "${ page.slug }" {
		t.Fatalf("expected nested span text to remain unrendered, got %q", text)
	}

	if bytes.Contains(htmlBytes, []byte("ht-apply")) {
		t.Fatalf("expected ht-apply attribute to be removed in tidy mode; got:\n%s", htmlBytes)
	}
}

func TestRenderValuesJavascriptPlugin(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<section ht-apply>
					${ page.title | upper }
				</section>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	upper, err := scripting.Function(`return input.toUpperCase()`, nil)
	if err != nil {
		t.Fatalf("error compiling scripting function: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"title": "Hello, HyperTemplates",
		},
	}
	template := &Template{
		Layout:   layout,
		TidyMode: true,
		Plugins: htvalue.Plugins{
			"upper": upper,
		},
	}

	htmlBytes, err := template.Process(&data, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("HELLO, HYPERTEMPLATES")) {
		t.Fatalf("expected JavaScript plugin output; got:\n%s", htmlBytes)
	}
}

func TestRenderValuesNested(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<section ht-apply class="${ page.class }">
					Hello ${ page.title }
					<div>
						<span ht-apply data-slug="${ page.slug }">${ page.title }</span>
					</div>
				</section>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"class": "hero",
			"slug":  "hello-hypertemplates",
			"title": "Hello, HyperTemplates",
		},
	}

	template := &Template{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := template.Process(&data, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	renderedLayout, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	section := Query(renderedLayout, "section.hero")
	if section == nil {
		t.Fatalf("expected outer ht-apply element to render attributes; got:\n%s", htmlBytes)
	}

	var directText string
	for child := section.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.TextNode {
			continue
		}
		if !strings.Contains(child.Data, "Hello, HyperTemplates") {
			continue
		}
		directText = strings.TrimSpace(child.Data)
		break
	}

	if directText != "Hello Hello, HyperTemplates" {
		t.Fatalf("expected rendered outer direct text %q, got %q", "Hello Hello, HyperTemplates", directText)
	}

	span := Query(renderedLayout, "section.hero div span")
	if span == nil {
		t.Fatalf("expected nested ht-apply element to render on its own pass; got:\n%s", htmlBytes)
	}

	if slug := getAttributeString(span, "data-slug"); slug != "hello-hypertemplates" {
		t.Fatalf("expected nested data-slug %q, got %q", "hello-hypertemplates", slug)
	}

	if text := strings.TrimSpace(span.FirstChild.Data); text != "Hello, HyperTemplates" {
		t.Fatalf("expected nested span text %q, got %q", "Hello, HyperTemplates", text)
	}

	if bytes.Contains(htmlBytes, []byte("ht-apply")) {
		t.Fatalf("expected nested ht-apply attributes to be removed; got:\n%s", htmlBytes)
	}
}

func TestRenderValuesStyleElement(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<style ht-apply>
					.hero { color: --ht-value("theme.color"); background: --ht-value('theme.bg'); }
				</style>
			</head>
			<body></body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"theme": TemplateData{
			"color": "red",
			"bg":    "black",
		},
	}

	template := &Template{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := template.Process(&data, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("color: red")) {
		t.Fatalf("expected <style> --ht-value(\"theme.color\") to render as red; got:\n%s", htmlBytes)
	}
	if !bytes.Contains(htmlBytes, []byte("background: black")) {
		t.Fatalf("expected <style> --ht-value('theme.bg') to render as black; got:\n%s", htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("--ht-value(")) {
		t.Fatalf("expected --ht-value(...) calls to be fully substituted; got:\n%s", htmlBytes)
	}
}

func TestRenderValuesStyleElementFallback(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<style ht-apply>
					.hero { color: --ht-value("theme.color", red); padding: --ht-value("theme.pad", 10px); }
				</style>
			</head>
			<body></body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{}

	template := &Template{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := template.Process(&data, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("color: red")) {
		t.Fatalf("expected fallback 'red' to render when theme.color is missing; got:\n%s", htmlBytes)
	}
	if !bytes.Contains(htmlBytes, []byte("padding: 10px")) {
		t.Fatalf("expected fallback '10px' to render when theme.pad is missing; got:\n%s", htmlBytes)
	}
}

func TestRenderValuesStyleAttribute(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<div ht-apply style='color: --ht-value("theme.color");'></div>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"theme": TemplateData{
			"color": "red",
		},
	}

	template := &Template{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := template.Process(&data, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	div := Query(layout, "div")
	if div == nil {
		t.Fatalf("expected rendered output to include div; got:\n%s", htmlBytes)
	}

	style := getAttributeString(div, "style")
	if style != "color: red;" {
		t.Fatalf("expected style=%q, got %q", "color: red;", style)
	}
}
