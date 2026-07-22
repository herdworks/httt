package variable

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func renderNodesString(t *testing.T, nodes []*html.Node) string {
	t.Helper()
	var buffer bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&buffer, node); err != nil {
			t.Fatalf("html.Render: %v", err)
		}
	}
	return buffer.String()
}

func divParent() *html.Node {
	return &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
}

func TestRenderNodesPlainTextAndInterpolation(t *testing.T) {
	data := testGetter{"page": map[string]any{"title": "Hello"}}
	nodes, err := RenderNodes("Title: ${ page.title }", data, divParent())
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	if got := renderNodesString(t, nodes); got != "Title: Hello" {
		t.Fatalf("rendered = %q, want Title: Hello", got)
	}
}

func TestRenderNodesParsesHTMLResults(t *testing.T) {
	data := testGetter{"page": map[string]any{"body": "<em>hi</em>"}}
	nodes, err := RenderNodes(`${ page.body }`, data, divParent())
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	if got := renderNodesString(t, nodes); got != "<em>hi</em>" {
		t.Fatalf("rendered = %q, want <em>hi</em>", got)
	}
}

func TestRenderNodesFunctionCall(t *testing.T) {
	funcs := FuncMap{"raw": func(value any) string { return value.(string) }}
	data := testGetter{"page": map[string]any{"body": "<strong>hi</strong>"}}
	nodes, err := RenderNodes(`Before ${ raw(page.body) } after`, data, divParent(), funcs)
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	if got := renderNodesString(t, nodes); got != "Before <strong>hi</strong> after" {
		t.Fatalf("rendered = %q", got)
	}
}

func TestRenderNodesParentContext(t *testing.T) {
	parent := &html.Node{Type: html.ElementNode, Data: "table", DataAtom: atom.Table}
	nodes, err := RenderNodes("<tr><td>cell</td></tr>", testGetter{}, parent)
	if err != nil {
		t.Fatalf("RenderNodes() error = %v", err)
	}
	got := renderNodesString(t, nodes)
	if !strings.Contains(got, "<tr>") || !strings.Contains(got, "<td>cell</td>") {
		t.Fatalf("table-context output = %q", got)
	}
}

func TestRenderNodesMissingValue(t *testing.T) {
	_, err := RenderNodes(`${ page.missing }`, testGetter{}, divParent())
	var missing *MissingValueError
	if !errors.As(err, &missing) {
		t.Fatalf("RenderNodes() error = %v, want MissingValueError", err)
	}
}
