package value

import (
	"bytes"
	htmltemplate "html/template"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func renderNodesString(t *testing.T, nodes []*html.Node) string {
	t.Helper()
	var buf bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&buf, node); err != nil {
			t.Fatalf("html.Render: %v", err)
		}
	}
	return buf.String()
}

func divParent() *html.Node {
	return &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
}

func TestRenderNodes_PlainText(t *testing.T) {
	nodes, err := RenderNodes("Hello world", testGetter{}, divParent())
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("RenderNodes() returned %d nodes, want 1", len(nodes))
	}
	if nodes[0].Type != html.TextNode {
		t.Fatalf("node type = %v, want TextNode", nodes[0].Type)
	}
	if nodes[0].Data != "Hello world" {
		t.Fatalf("text = %q, want %q", nodes[0].Data, "Hello world")
	}
}

func TestRenderNodes_KeypathSubstitution(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": "Hello"}}
	nodes, err := RenderNodes("Title: ${ page.title }", data, divParent())
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	got := renderNodesString(t, nodes)
	if got != "Title: Hello" {
		t.Fatalf("rendered = %q, want %q", got, "Title: Hello")
	}
}

func TestRenderNodes_AutoEscapesUserData(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": "<script>alert(1)</script>"}}
	nodes, err := RenderNodes("Title: ${ page.title }", data, divParent())
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	if len(nodes) != 1 || nodes[0].Type != html.TextNode {
		t.Fatalf("expected single text node, got %d nodes", len(nodes))
	}
	if nodes[0].Data != "Title: <script>alert(1)</script>" {
		t.Fatalf("text node data = %q, want literal angle brackets (escaped at serialize time)", nodes[0].Data)
	}
	// Serialized output must escape the script tag — i.e., no live <script> element in the tree.
	got := renderNodesString(t, nodes)
	if strings.Contains(got, "<script>") {
		t.Fatalf("serialized output contains live <script> element: %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("serialized output missing escaped script tag: %q", got)
	}
}

func TestRenderNodes_TemplateHTMLPassesThrough(t *testing.T) {
	funcs := FuncMap{
		"raw": func(value any) htmltemplate.HTML {
			return htmltemplate.HTML(value.(string))
		},
	}
	data := testGetter{"page": map[string]any{"body": "<em>hi</em>"}}
	nodes, err := RenderNodes(`${ page.body | raw }`, data, divParent(), funcs)
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	got := renderNodesString(t, nodes)
	if got != "<em>hi</em>" {
		t.Fatalf("rendered = %q, want %q", got, "<em>hi</em>")
	}
	// Confirm we got a real <em> element, not an escaped text node.
	if len(nodes) != 1 || nodes[0].Type != html.ElementNode || nodes[0].Data != "em" {
		t.Fatalf("expected single <em> ElementNode, got %d nodes", len(nodes))
	}
}

func TestRenderNodes_MixedTextAndTemplateHTML(t *testing.T) {
	funcs := FuncMap{
		"raw": func(value any) htmltemplate.HTML {
			return htmltemplate.HTML(value.(string))
		},
	}
	data := testGetter{"page": map[string]any{"body": "<em>hi</em>"}}
	nodes, err := RenderNodes(`Hello ${ page.body | raw } world`, data, divParent(), funcs)
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	got := renderNodesString(t, nodes)
	if got != "Hello <em>hi</em> world" {
		t.Fatalf("rendered = %q, want %q", got, "Hello <em>hi</em> world")
	}
}

func TestRenderNodes_ParentContextHonored(t *testing.T) {
	// ParseFragment with a <table> parent honors HTML5 table-context parsing
	// (wraps <tr> in implicit <tbody>); with a <div> parent it would strip the
	// <tr> entirely.
	tableParent := &html.Node{Type: html.ElementNode, Data: "table", DataAtom: atom.Table}
	nodes, err := RenderNodes("<tr><td>cell</td></tr>", testGetter{}, tableParent)
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	got := renderNodesString(t, nodes)
	if !strings.Contains(got, "<tr>") || !strings.Contains(got, "<td>cell</td>") {
		t.Fatalf("expected table-context parse to preserve <tr>/<td>; got: %q", got)
	}

	// Sanity check: the same fragment parsed under a <div> parent strips the row.
	divNodes, err := RenderNodes("<tr><td>cell</td></tr>", testGetter{}, divParent())
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	divOut := renderNodesString(t, divNodes)
	if strings.Contains(divOut, "<tr>") {
		t.Fatalf("expected div-context parse to strip <tr>; got: %q", divOut)
	}
}

func TestRenderNodes_MissingKeypathRestoresSource(t *testing.T) {
	source := `${ page.missing }`
	nodes, err := RenderNodes(source, testGetter{}, divParent())
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	got := renderNodesString(t, nodes)
	if got != source {
		t.Fatalf("rendered = %q, want literal source %q", got, source)
	}
}

func TestRenderNodes_UserFuncStillWorks(t *testing.T) {
	funcs := FuncMap{"upper": strings.ToUpper}
	data := testGetter{"page": map[string]any{"title": "hello"}}
	nodes, err := RenderNodes(`${ page.title | upper }`, data, divParent(), funcs)
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	got := renderNodesString(t, nodes)
	if got != "HELLO" {
		t.Fatalf("rendered = %q, want %q", got, "HELLO")
	}
}

func TestRenderNodes_BuiltinPipeRejected(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"printf", `${ page.title | printf "%v" }`},
		{"len", `${ page.title | len }`},
		{"index", `${ page.items | index 0 }`},
		{"print", `${ page.title | print }`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := testGetter{"page": map[string]any{"title": "x"}}
			_, err := RenderNodes(tc.source, data, divParent())
			if err == nil {
				t.Fatalf("RenderNodes() error = nil, want function-not-registered error")
			}
			if !strings.Contains(err.Error(), "not registered") {
				t.Fatalf("RenderNodes() error = %v, want function-not-registered error", err)
			}
		})
	}
}

func TestRewrite_ImplicitGetUnchanged(t *testing.T) {
	// Regression-lock: implicit get rewrite must still happen after the
	// rewriter hardening.
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"bare keypath", `${ page.title }`, `${ get "page.title" }`},
		{"comma fallback", `${ site.title, page.title, "Untitled" }`, `${ get "site.title, page.title" "Untitled" }`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rewrite(tc.source, SyntaxDefault)
			if err != nil {
				t.Fatalf("rewrite() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("rewrite() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRewrite_ImplicitGetWithUserFuncTail(t *testing.T) {
	// Regression-lock: implicit get under a registered user-func pipe still
	// rewrites correctly.
	funcs := FuncMap{"upper": strings.ToUpper}
	got, err := rewriteWithFuncs(`${ page.title | upper }`, SyntaxDefault, funcs)
	if err != nil {
		t.Fatalf("rewriteWithFuncs() error = %v", err)
	}
	want := `${ get "page.title" | upper }`
	if got != want {
		t.Fatalf("rewriteWithFuncs() = %q, want %q", got, want)
	}
}
