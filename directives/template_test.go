package directives

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/scripting"
	httheme "github.com/herdworks/hypertemplates-go/theme"
	htvariable "github.com/herdworks/hypertemplates-go/variable"
	"golang.org/x/net/html"
)

// Template.ResolveIncludes
// Ensure that ht-include elements are properly resolved.
func TestResolveIncludes(t *testing.T) {
	tests := []struct {
		name    string
		layout  string
		theme   string
		err     error
		missing bool
	}{
		{
			name:   "resolve includes",
			layout: "testdata/ht-include/layouts/ok.html",
			theme:  "testdata/ht-include/theme.json",
			err:    nil,
		},
		{
			name:    "broken include",
			layout:  "testdata/ht-include/layouts/broken.html",
			theme:   "testdata/ht-include/theme.json",
			err:     ErrIncludeFragment,
			missing: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var layoutBytes []byte
			var layoutHTML *html.Node
			var theme *httheme.Theme = &httheme.Theme{URI: &url.URL{Path: tc.theme}}
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
			template := &document.Document{Layout: layoutHTML, Theme: theme}
			if template.Layout == nil {
				t.Fatalf("template.Layout is nil")
			}
			err = ResolveIncludes(template)
			if tc.err == nil && err != nil {
				t.Fatalf("error resolving includes: %v", err)
			}
			if tc.err == nil {
				return
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected error %q, got: %v", tc.err, err)
			}
			if tc.missing && !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("expected missing include error to wrap fs.ErrNotExist, got: %v", err)
			}
		})
	}
}

func TestResolveIncludesRejectsUnsafeReferences(t *testing.T) {
	tests := []struct {
		name string
		ref  string
	}{
		{name: "absolute", ref: "/etc/passwd"},
		{name: "parent traversal", ref: "../secret.html"},
		{name: "empty", ref: ""},
		{name: "backslash separated", ref: `partials\header.html`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := setupBuiltinTest(t, `<html><body><div ht-include></div></body></html>`, nil)
			include := document.Query(doc.Layout, "[ht-include]")
			if include == nil {
				t.Fatal("include placeholder not found")
			}
			document.SetAttribute(include, "ht-include", tc.ref)

			err := ResolveIncludes(doc)
			if !errors.Is(err, ErrIncludeFragment) {
				t.Fatalf("expected ErrIncludeFragment for %q, got: %v", tc.ref, err)
			}
			if errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("expected %q to fail path validation, got a missing-file error: %v", tc.ref, err)
			}
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
			var theme *httheme.Theme = &httheme.Theme{URI: &url.URL{Path: tc.theme}}
			var layoutBytes []byte
			var layoutHTML *html.Node
			var err error
			if layoutBytes, err = os.ReadFile(tc.layout); err != nil {
				t.Fatalf("error reading layout %q bytes: %v", tc.layout, err)
			}
			if layoutHTML, err = html.Parse(bytes.NewReader(layoutBytes)); err != nil {
				t.Fatalf("error parsing layout %q html: %v", tc.layout, err)
			}
			template := &document.Document{Layout: layoutHTML, Theme: theme}
			if template.Layout == nil {
				t.Fatalf("template.Layout is nil")
			}
			if err = ResolveIncludes(template); err != nil {
				t.Fatalf("error resolving includes: %v", err)
			}
			if matches := document.QueryAll(template.Layout, "header#hero"); len(matches) != 1 {
				t.Fatalf("expected 1 match for header#hero, got %d", len(matches))
			}
			if matches := document.QueryAll(template.Layout, "footer#forwarded"); len(matches) != 1 {
				t.Fatalf("expected 1 match for footer#forwarded, got %d", len(matches))
			}
		})
	}
}

// Conditional parser, truthiness, and evaluation coverage lives in
// condition_test.go alongside the active ht-if implementation.

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
			var theme *httheme.Theme
			var layout *html.Node
			var data TemplateData
			var template *document.Document
			var htmlBytes []byte
			var err error

			// configure the theme
			theme = &httheme.Theme{URI: &url.URL{Path: tc.theme}}
			theme.ResolveConfig()

			// load the layout
			if layout, err = theme.ResourceHTML(tc.layout); err != nil {
				t.Fatalf("error loading layout %q: %v", tc.layout, err)
			}

			// initialize the template
			template = &document.Document{
				Theme:          theme,
				Layout:         layout,
				TidyMode:       true,
				MarkdownConfig: nil,
			}

			// parse the template data
			if data, err = readTemplateDataFile(tc.data); err != nil {
				t.Fatalf("error parsing template data file %q: %v", tc.data, err)
			}

			// render the template
			if htmlBytes, err = Process(template, &data, nil, nil); err != nil {
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
	theme := &httheme.Theme{URI: &url.URL{Path: "testdata/ht-block/theme.json"}}
	theme.ResolveConfig()

	layout, err := theme.ResourceHTML("layouts/ok.html")
	if err != nil {
		t.Fatalf("error loading layout: %v", err)
	}

	pageData, err := readTemplateDataFile("testdata/ht-block/content/attachment.md")
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

	template := &document.Document{
		Theme:          theme,
		Layout:         layout,
		TidyMode:       true,
		MarkdownConfig: nil,
	}

	// Mirror the caller: outer data may or may not have page.*, but userData
	// wraps the page data under the "page" key explicitly.
	userData := &TemplateData{"page": pageData}

	htmlBytes, err := Process(template, &pageData, userData, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}
	t.Logf("\n%s\n", string(htmlBytes))

	if !bytes.Contains(htmlBytes, []byte("anker.com/products/a2345")) {
		t.Errorf("expected rendered output to include page.attachments.1.data.hypergraph.href; got:\n%s", htmlBytes)
	}
}

func TestRenderVariables(t *testing.T) {
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

	template := &document.Document{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	renderedLayout, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	section := document.Query(renderedLayout, "section")
	if section == nil {
		t.Fatalf("expected rendered output to include section; got:\n%s", htmlBytes)
	}

	if href := document.GetAttributeString(section, "data-href"); href != "https://example.com/posts/hello" {
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

	span := document.Query(renderedLayout, "section span")
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

func TestRenderVariablesNakedKeypathsRemainLiteral(t *testing.T) {
	tests := []struct {
		name       string
		apply      string
		strictMode bool
	}{
		{name: "non-strict alias", apply: "ht-apply"},
		{name: "strict alias", apply: "data-ht-apply", strictMode: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			layout, err := html.Parse(strings.NewReader(
				`<html><body><div ` + tc.apply + ` data-source="page.sentinel" style="color: page.sentinel">page.sentinel</div></body></html>`,
			))
			if err != nil {
				t.Fatalf("error parsing layout html: %v", err)
			}

			data := TemplateData{
				"page": TemplateData{"sentinel": "resolved-sentinel"},
			}
			template := &document.Document{
				Layout:     layout,
				StrictMode: tc.strictMode,
				TidyMode:   true,
			}
			htmlBytes, err := Process(template, &data, nil, nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			rendered, err := html.Parse(bytes.NewReader(htmlBytes))
			if err != nil {
				t.Fatalf("error parsing rendered html: %v", err)
			}
			div := document.Query(rendered, "div")
			if div == nil {
				t.Fatalf("expected rendered output to include div; got:\n%s", htmlBytes)
			}
			if document.HasAttribute(div, HyperApplyAttributeStrings...) {
				t.Fatalf("expected %s to be processed and removed; got:\n%s", tc.apply, htmlBytes)
			}
			if got := document.GetAttributeString(div, "data-source"); got != "page.sentinel" {
				t.Fatalf("data-source = %q, want unchanged naked keypath", got)
			}
			if got := document.GetAttributeString(div, "style"); got != "color: page.sentinel" {
				t.Fatalf("style = %q, want unchanged naked keypath", got)
			}
			if div.FirstChild == nil || div.FirstChild.Data != "page.sentinel" {
				got := ""
				if div.FirstChild != nil {
					got = div.FirstChild.Data
				}
				t.Fatalf("text content = %q, want unchanged naked keypath", got)
			}
		})
	}
}

func TestRenderVariablesJavascriptPlugin(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<section ht-apply>
					${ upper(page.title) }
				</section>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	upperPlugin := scripting.Plugin{
		Source: `
			export default function(input) {
				return input.toUpperCase()
			}
		`,
	}
	upperCallable, err := upperPlugin.Compile(nil)
	if err != nil {
		t.Fatalf("error compiling scripting plugin: %v", err)
	}
	upper := func(args ...any) (any, error) {
		return upperCallable(context.Background(), nil, args...)
	}

	data := TemplateData{
		"page": TemplateData{
			"title": "Hello, HyperTemplates",
		},
	}
	template := &document.Document{
		Layout:   layout,
		TidyMode: true,
		Plugins: htvariable.Plugins{
			"upper": {Name: "upper", Func: upper},
		},
	}

	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("HELLO, HYPERTEMPLATES")) {
		t.Fatalf("expected JavaScript plugin output; got:\n%s", htmlBytes)
	}
}

func TestRenderVariablesNested(t *testing.T) {
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

	template := &document.Document{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	renderedLayout, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	section := document.Query(renderedLayout, "section.hero")
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

	span := document.Query(renderedLayout, "section.hero div span")
	if span == nil {
		t.Fatalf("expected nested ht-apply element to render on its own pass; got:\n%s", htmlBytes)
	}

	if slug := document.GetAttributeString(span, "data-slug"); slug != "hello-hypertemplates" {
		t.Fatalf("expected nested data-slug %q, got %q", "hello-hypertemplates", slug)
	}

	if text := strings.TrimSpace(span.FirstChild.Data); text != "Hello, HyperTemplates" {
		t.Fatalf("expected nested span text %q, got %q", "Hello, HyperTemplates", text)
	}

	if bytes.Contains(htmlBytes, []byte("ht-apply")) {
		t.Fatalf("expected nested ht-apply attributes to be removed; got:\n%s", htmlBytes)
	}
}

func TestRenderVariablesStyleElement(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<style ht-apply>
					.hero { color: --ht-var("theme.color"); background: --ht-var('theme.bg'); }
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

	template := &document.Document{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("color: red")) {
		t.Fatalf("expected <style> --ht-var(\"theme.color\") to render as red; got:\n%s", htmlBytes)
	}
	if !bytes.Contains(htmlBytes, []byte("background: black")) {
		t.Fatalf("expected <style> --ht-var('theme.bg') to render as black; got:\n%s", htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("--ht-var(")) {
		t.Fatalf("expected --ht-var(...) calls to be fully substituted; got:\n%s", htmlBytes)
	}
}

func TestRenderVariablesStyleElementFallback(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<head>
				<style ht-apply>
						.hero { color: --ht-var("theme.color", red); padding: --ht-var("theme.pad", 10px); }
				</style>
			</head>
			<body></body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{}

	template := &document.Document{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := Process(template, &data, nil, nil)
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

func TestRenderVariablesStyleAttribute(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<div ht-apply style='color: --ht-var("theme.color");'></div>
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

	template := &document.Document{
		Layout:   layout,
		TidyMode: true,
	}

	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	div := document.Query(layout, "div")
	if div == nil {
		t.Fatalf("expected rendered output to include div; got:\n%s", htmlBytes)
	}

	style := document.GetAttributeString(div, "style")
	if style != "color: red;" {
		t.Fatalf("expected style=%q, got %q", "color: red;", style)
	}
}

func TestRenderVariablesTextNodeIsParsedAsHTML(t *testing.T) {
	// Text-node output is always parsed as HTML — no auto-escaping. A bare
	// keypath whose value is HTML markup gets inserted as real elements.
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<div ht-apply>${ page.body }</div>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"body": "<em>hi</em>",
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("<em>hi</em>")) {
		t.Fatalf("expected <em>hi</em> in output, got:\n%s", htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("&lt;em&gt;")) {
		t.Fatalf("expected raw <em> insertion, got escaped output:\n%s", htmlBytes)
	}
}

func TestRenderVariablesMarkdownFunc(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<article ht-apply>${ markdown(page.body) }</article>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"body": "# Title\n\nA paragraph.",
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("<h1")) {
		t.Fatalf("expected <h1> from markdown rendering, got:\n%s", htmlBytes)
	}
	if !bytes.Contains(htmlBytes, []byte("<p>A paragraph.</p>")) {
		t.Fatalf("expected <p>A paragraph.</p>, got:\n%s", htmlBytes)
	}
}

func TestRenderVariablesTextFuncEscapesHTML(t *testing.T) {
	// `${ text(x) }` HTML-escapes the value so author data containing `<`,
	// `>`, etc. renders as literal text rather than markup. Round-trips
	// through text/template (escape) → html.ParseFragment (decode entities
	// into text-node chars) → html.Render (re-escape on serialize).
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<h1 ht-apply>${ text(page.title) }</h1>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"title": "<script>alert(1)</script>",
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if bytes.Contains(htmlBytes, []byte("<script>alert(1)")) {
		t.Fatalf("expected escaped output, got live <script> tag:\n%s", htmlBytes)
	}
	if !bytes.Contains(htmlBytes, []byte("&lt;script&gt;")) {
		t.Fatalf("expected &lt;script&gt; in output, got:\n%s", htmlBytes)
	}
}

func TestRenderVariablesTextFuncWithPlainText(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<h1 ht-apply>${ text(page.title) }</h1>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"title": "Hello world",
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("Hello world")) {
		t.Fatalf("expected plain text in output, got:\n%s", htmlBytes)
	}
}

func TestRenderVariablesTextFuncNilReturnsMissingValueError(t *testing.T) {
	// Built-in fallback selection happens before text is invoked. An all-nil
	// fallback chain therefore has the same missing-value behavior as get.
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<h1 ht-apply>${ text(page.missing) }</h1>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	_, err = Process(template, &TemplateData{}, nil, nil)
	var missing *htvariable.MissingValueError
	if !errors.As(err, &missing) {
		t.Fatalf("Process() error = %v, want MissingValueError", err)
	}
}

func TestRenderVariablesUserDataInterpretedAsHTML(t *testing.T) {
	// Authors own their data; we no longer auto-escape. Markup in data is
	// interpreted as live HTML elements when injected into a text node.
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<h1 ht-apply>${ page.title }</h1>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"title": "<em>hi</em>",
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("<em>hi</em>")) {
		t.Fatalf("expected live <em>hi</em> in output, got:\n%s", htmlBytes)
	}
}

func TestRenderVariablesMissingErrorIncludesElementContext(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(
		`<html><body><a ht-apply id="home" class="nav" href="${ page.missing }" target="_blank">link</a></body></html>`,
	))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	_, err = Process(template, &TemplateData{}, nil, nil)
	if err == nil {
		t.Fatal("Process() error = nil, want MissingValueError wrapped with element context")
	}

	var miss *htvariable.MissingValueError
	if !errors.As(err, &miss) {
		t.Fatalf("Process() error = %v, want errors.As(MissingValueError) to succeed", err)
	}
	if miss.Source != `${ page.missing }` {
		t.Fatalf("MissingValueError.Source = %q, want %q", miss.Source, `${ page.missing }`)
	}

	msg := err.Error()
	// Full element (with children) is included after the message so the
	// failing variable can be traced back to its source location, regardless
	// of whether it failed in an attribute or a text node.
	for _, want := range []string{
		`failed rendering variables in attribute "href"`,
		`a template variable in ${ page.missing } didn't return a value`,
		`<a`,
		`ht-apply`,
		`id="home"`,
		`class="nav"`,
		`href="${ page.missing }"`,
		`target="_blank"`,
		`>link</a>`,
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error message missing %q:\n%s", want, msg)
		}
	}
}

func TestRenderVariablesMissingErrorIncludesElementForTextNode(t *testing.T) {
	// When the failing variable is in a text node (not an attribute), the
	// full element render is what reveals the failing variable.
	layout, err := html.Parse(strings.NewReader(
		`<html><body><h1 ht-apply id="title">Welcome, ${ page.username }!</h1></body></html>`,
	))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	_, err = Process(template, &TemplateData{}, nil, nil)
	if err == nil {
		t.Fatal("Process() error = nil, want MissingValueError")
	}

	msg := err.Error()
	for _, want := range []string{
		`failed rendering variables in element text content`,
		`a template variable in Welcome, ${ page.username }! didn't return a value`,
		`<h1`,
		`id="title"`,
		`Welcome, ${ page.username }!`,
		`</h1>`,
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error message missing %q:\n%s", want, msg)
		}
	}
}

func TestRenderVariablesErrorsIncludeLocationAndElement(t *testing.T) {
	tests := []struct {
		name     string
		layout   string
		location string
	}{
		{
			name:     "attribute",
			layout:   `<html><body><a ht-apply id="target" href="${ unknown(page.title) }">link</a></body></html>`,
			location: `attribute "href"`,
		},
		{
			name:     "style text",
			layout:   `<html><head><style ht-apply id="target">a { color: --ht-var("unknown(page.color)"); }</style></head></html>`,
			location: `<style> text content`,
		},
		{
			name:     "element text",
			layout:   `<html><body><h1 ht-apply id="target">${ unknown(page.title) }</h1></body></html>`,
			location: `element text content`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			layout, err := html.Parse(strings.NewReader(tc.layout))
			if err != nil {
				t.Fatalf("error parsing layout html: %v", err)
			}

			template := &document.Document{Layout: layout, TidyMode: true}
			_, err = Process(template, &TemplateData{}, nil, nil)
			if err == nil {
				t.Fatal("Process() error = nil")
			}
			for _, want := range []string{
				"failed rendering variables in " + tc.location,
				`function "unknown" not registered`,
				`id="target"`,
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error message missing %q:\n%s", want, err)
				}
			}
		})
	}
}

func TestRenderContentHtmlMarkdownRefactorRegression(t *testing.T) {
	// Regression-lock: RenderContent's html: and markdown: directive
	// branches still produce the same output after the helper extraction.
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<div ht-content="html:page.body"></div>
				<article ht-content="markdown:page.markdown"></article>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"body":     "<em>raw</em>",
			"markdown": "# Heading",
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if !bytes.Contains(htmlBytes, []byte("<em>raw</em>")) {
		t.Fatalf("html: branch missing expected <em>raw</em>; got:\n%s", htmlBytes)
	}
	if !bytes.Contains(htmlBytes, []byte("<h1")) {
		t.Fatalf("markdown: branch missing expected <h1>; got:\n%s", htmlBytes)
	}
}

// TestImplicitApplyOnTemplateAttribute exercises the ergonomic shortcut where
// ${ ... } expressions on an ht-template element's own attributes resolve
// against the per-iteration scope without an explicit ht-apply.
func TestImplicitApplyOnTemplateAttribute(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<a ht-template="link:page.links" href="${ link.href }"></a>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"links": TemplateDataSet{
				TemplateData{"href": "/a"},
				TemplateData{"href": "/b"},
			},
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	anchors := document.QueryAll(rendered, "body > a[href]")
	if len(anchors) != 2 {
		t.Fatalf("expected 2 rendered <a> siblings, got %d; output:\n%s", len(anchors), htmlBytes)
	}
	want := []string{"/a", "/b"}
	for i, a := range anchors {
		if href := document.GetAttributeString(a, "href"); href != want[i] {
			t.Fatalf("anchor %d: expected href=%q, got %q", i, want[i], href)
		}
	}

	if bytes.Contains(htmlBytes, []byte("data-ht-apply")) || bytes.Contains(htmlBytes, []byte("ht-apply")) {
		t.Fatalf("synthetic ht-apply attribute leaked into output:\n%s", htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("${")) {
		t.Fatalf("expected ${ ... } expressions to be rendered; got:\n%s", htmlBytes)
	}
}

// TestImplicitApplyOnTemplateText verifies that ${ ... } in the direct text
// children of an ht-template element resolve without an explicit ht-apply.
func TestImplicitApplyOnTemplateText(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<span ht-template="item:items">${ item }</span>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{"items": []any{"x", "y"}}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	spans := document.QueryAll(rendered, "body > span")
	if len(spans) != 2 {
		t.Fatalf("expected 2 rendered <span> siblings, got %d; output:\n%s", len(spans), htmlBytes)
	}
	want := []string{"x", "y"}
	for i, s := range spans {
		if s.FirstChild == nil || strings.TrimSpace(s.FirstChild.Data) != want[i] {
			got := ""
			if s.FirstChild != nil {
				got = strings.TrimSpace(s.FirstChild.Data)
			}
			t.Fatalf("span %d: expected text %q, got %q", i, want[i], got)
		}
	}
}

// TestImplicitApplyDoesNotClobberExplicit confirms that an author-supplied
// ht-apply on the same element is preserved through iteration (the synthetic
// injection is a no-op when ht-apply is already present).
func TestImplicitApplyDoesNotClobberExplicit(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<a ht-template="link:page.links" ht-apply href="${ link.href }"></a>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"links": TemplateDataSet{
				TemplateData{"href": "/a"},
				TemplateData{"href": "/b"},
			},
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	anchors := document.QueryAll(rendered, "body > a[href]")
	if len(anchors) != 2 {
		t.Fatalf("expected 2 rendered <a> siblings, got %d; output:\n%s", len(anchors), htmlBytes)
	}
	want := []string{"/a", "/b"}
	for i, a := range anchors {
		if href := document.GetAttributeString(a, "href"); href != want[i] {
			t.Fatalf("anchor %d: expected href=%q, got %q", i, want[i], href)
		}
	}
}

// TestImplicitApplyStrictMode exercises the data- attribute form under
// StrictMode so the synthetic data-ht-apply is picked up by the strict
// ApplyDirective selector.
func TestImplicitApplyStrictMode(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<a data-ht-template="link:page.links" href="${ link.href }"></a>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"links": TemplateDataSet{
				TemplateData{"href": "/a"},
				TemplateData{"href": "/b"},
			},
		},
	}

	template := &document.Document{Layout: layout, TidyMode: true, StrictMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	anchors := document.QueryAll(rendered, "body > a[href]")
	if len(anchors) != 2 {
		t.Fatalf("expected 2 rendered <a> siblings, got %d; output:\n%s", len(anchors), htmlBytes)
	}
	want := []string{"/a", "/b"}
	for i, a := range anchors {
		if href := document.GetAttributeString(a, "href"); href != want[i] {
			t.Fatalf("anchor %d: expected href=%q, got %q", i, want[i], href)
		}
	}

	if bytes.Contains(htmlBytes, []byte("data-ht-apply")) {
		t.Fatalf("synthetic data-ht-apply attribute leaked into strict-mode output:\n%s", htmlBytes)
	}
}

// TestImplicitApplyCleanupNonTidy verifies that the synthetic ht-apply
// attribute is stripped from cloned elements regardless of TidyMode, so it
// never reaches the serialized output.
func TestImplicitApplyCleanupNonTidy(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<a ht-template="link:page.links" href="${ link.href }"></a>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{
		"page": TemplateData{
			"links": TemplateDataSet{
				TemplateData{"href": "/a"},
			},
		},
	}

	template := &document.Document{Layout: layout, TidyMode: false}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if bytes.Contains(htmlBytes, []byte("data-ht-apply")) {
		t.Fatalf("synthetic data-ht-apply leaked into non-tidy output:\n%s", htmlBytes)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	clones := document.QueryAll(rendered, "body > a[href]")
	found := false
	for _, a := range clones {
		if document.GetAttributeString(a, "href") == "/a" && !document.HasAttribute(a, "ht-template", "data-ht-template") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an iterated <a href=\"/a\"> sibling without ht-template; got:\n%s", htmlBytes)
	}
}

// TestImplicitApplyNoMarker confirms that an ht-template element without any
// ${ ... } expressions iterates without panic; the synthetic RenderVariables
// pass is a no-op and no stray attribute is emitted.
func TestImplicitApplyNoMarker(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<li ht-template="x:items">static</li>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	data := TemplateData{"items": []any{"a", "b", "c"}}

	template := &document.Document{Layout: layout, TidyMode: true}
	htmlBytes, err := Process(template, &data, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	items := document.QueryAll(rendered, "body > li")
	if len(items) != 3 {
		t.Fatalf("expected 3 <li> siblings, got %d; output:\n%s", len(items), htmlBytes)
	}
	for i, li := range items {
		if li.FirstChild == nil || strings.TrimSpace(li.FirstChild.Data) != "static" {
			t.Fatalf("li %d: expected text %q, got %q", i, "static", strings.TrimSpace(li.FirstChild.Data))
		}
	}
	if bytes.Contains(htmlBytes, []byte("data-ht-apply")) || bytes.Contains(htmlBytes, []byte("ht-apply")) {
		t.Fatalf("synthetic ht-apply attribute leaked into output:\n%s", htmlBytes)
	}
}

// TestImplicitApplyOnBlockText verifies that ${ ... } in the direct text
// children of an ht-block fragment root resolve without an explicit ht-apply.
func TestImplicitApplyOnBlockText(t *testing.T) {
	theme := &httheme.Theme{URI: &url.URL{Path: "testdata/ht-block/theme.json"}}
	theme.ResolveConfig()

	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<ia-text ht-block title="hello"></ia-text>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	template := &document.Document{Layout: layout, Theme: theme, TidyMode: true}
	htmlBytes, err := Process(template, &TemplateData{}, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	matches := document.QueryAll(rendered, "body ia-text")
	if len(matches) != 1 {
		t.Fatalf("expected 1 rendered <ia-text>, got %d; output:\n%s", len(matches), htmlBytes)
	}
	if matches[0].FirstChild == nil || strings.TrimSpace(matches[0].FirstChild.Data) != "hello" {
		got := ""
		if matches[0].FirstChild != nil {
			got = strings.TrimSpace(matches[0].FirstChild.Data)
		}
		t.Fatalf("expected text %q, got %q; output:\n%s", "hello", got, htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("data-ht-apply")) || bytes.Contains(htmlBytes, []byte("ht-apply")) {
		t.Fatalf("synthetic ht-apply attribute leaked into output:\n%s", htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("${")) {
		t.Fatalf("expected ${ ... } expressions to be rendered; got:\n%s", htmlBytes)
	}
}

// TestImplicitApplyOnBlockAttribute verifies that ${ ... } expressions on an
// ht-block fragment root's own attributes resolve without an explicit ht-apply.
func TestImplicitApplyOnBlockAttribute(t *testing.T) {
	theme := &httheme.Theme{URI: &url.URL{Path: "testdata/ht-block/theme.json"}}
	theme.ResolveConfig()

	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<ia-attr ht-block title="hello"></ia-attr>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	template := &document.Document{Layout: layout, Theme: theme, TidyMode: true}
	htmlBytes, err := Process(template, &TemplateData{}, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	matches := document.QueryAll(rendered, "body ia-attr[data-x]")
	if len(matches) != 1 {
		t.Fatalf("expected 1 rendered <ia-attr> with data-x, got %d; output:\n%s", len(matches), htmlBytes)
	}
	if got := document.GetAttributeString(matches[0], "data-x"); got != "hello" {
		t.Fatalf("expected data-x=%q, got %q; output:\n%s", "hello", got, htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("data-ht-apply")) || bytes.Contains(htmlBytes, []byte("ht-apply")) {
		t.Fatalf("synthetic ht-apply attribute leaked into output:\n%s", htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("${")) {
		t.Fatalf("expected ${ ... } expressions to be rendered; got:\n%s", htmlBytes)
	}
}

// TestImplicitApplyOnBlockDoesNotClobberExplicit confirms that an
// author-supplied ht-apply on the block fragment root is preserved (the
// synthetic injection is a no-op when ht-apply is already present).
func TestImplicitApplyOnBlockDoesNotClobberExplicit(t *testing.T) {
	theme := &httheme.Theme{URI: &url.URL{Path: "testdata/ht-block/theme.json"}}
	theme.ResolveConfig()

	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<ia-explicit ht-block title="hello"></ia-explicit>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	template := &document.Document{Layout: layout, Theme: theme, TidyMode: true}
	htmlBytes, err := Process(template, &TemplateData{}, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	matches := document.QueryAll(rendered, "body ia-explicit")
	if len(matches) != 1 {
		t.Fatalf("expected 1 rendered <ia-explicit>, got %d; output:\n%s", len(matches), htmlBytes)
	}
	if matches[0].FirstChild == nil || strings.TrimSpace(matches[0].FirstChild.Data) != "hello" {
		got := ""
		if matches[0].FirstChild != nil {
			got = strings.TrimSpace(matches[0].FirstChild.Data)
		}
		t.Fatalf("expected text %q, got %q; output:\n%s", "hello", got, htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("${")) {
		t.Fatalf("expected ${ ... } expressions to be rendered; got:\n%s", htmlBytes)
	}
}

// TestImplicitApplyOnBlockNoMarker confirms that a block fragment without any
// ${ ... } expressions renders cleanly; the synthetic RenderVariables pass is a
// no-op and no stray ht-apply attribute is emitted.
func TestImplicitApplyOnBlockNoMarker(t *testing.T) {
	theme := &httheme.Theme{URI: &url.URL{Path: "testdata/ht-block/theme.json"}}
	theme.ResolveConfig()

	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<ia-static ht-block></ia-static>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	template := &document.Document{Layout: layout, Theme: theme, TidyMode: true}
	htmlBytes, err := Process(template, &TemplateData{}, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	rendered, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("error parsing rendered html: %v", err)
	}

	matches := document.QueryAll(rendered, "body ia-static")
	if len(matches) != 1 {
		t.Fatalf("expected 1 rendered <ia-static>, got %d; output:\n%s", len(matches), htmlBytes)
	}
	if matches[0].FirstChild == nil || strings.TrimSpace(matches[0].FirstChild.Data) != "just static text" {
		got := ""
		if matches[0].FirstChild != nil {
			got = strings.TrimSpace(matches[0].FirstChild.Data)
		}
		t.Fatalf("expected text %q, got %q; output:\n%s", "just static text", got, htmlBytes)
	}
	if bytes.Contains(htmlBytes, []byte("data-ht-apply")) || bytes.Contains(htmlBytes, []byte("ht-apply")) {
		t.Fatalf("synthetic ht-apply attribute leaked into output:\n%s", htmlBytes)
	}
}

// TestImplicitApplyOnBlockCleanupNonTidy verifies that the synthetic ht-apply
// attribute is stripped from the block fragment regardless of TidyMode, so it
// never reaches the serialized output.
func TestImplicitApplyOnBlockCleanupNonTidy(t *testing.T) {
	theme := &httheme.Theme{URI: &url.URL{Path: "testdata/ht-block/theme.json"}}
	theme.ResolveConfig()

	layout, err := html.Parse(strings.NewReader(`
		<html>
			<body>
				<ia-text ht-block title="hello"></ia-text>
			</body>
		</html>
	`))
	if err != nil {
		t.Fatalf("error parsing layout html: %v", err)
	}

	template := &document.Document{Layout: layout, Theme: theme, TidyMode: false}
	htmlBytes, err := Process(template, &TemplateData{}, nil, nil)
	if err != nil {
		t.Fatalf("error processing template: %v", err)
	}

	if bytes.Contains(htmlBytes, []byte("data-ht-apply")) {
		t.Fatalf("synthetic data-ht-apply leaked into non-tidy output:\n%s", htmlBytes)
	}
}
