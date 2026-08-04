package hypertemplates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renderTestLayout(t *testing.T, layout, data string, untidy bool) string {
	t.Helper()
	root := t.TempDir()
	layoutPath := filepath.Join(root, "layout.html")
	dataPath := filepath.Join(root, "data.json")
	if err := os.WriteFile(layoutPath, []byte(layout), 0644); err != nil {
		t.Fatalf("write layout: %v", err)
	}
	if err := os.WriteFile(dataPath, []byte(data), 0644); err != nil {
		t.Fatalf("write data: %v", err)
	}
	rendered, err := Render(dataPath, "layout.html", filepath.Join(root, "theme.json"), untidy, &DefaultHyperMarkConfig, nil)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return string(rendered)
}

func TestRenderFragmentWithArbitraryData(t *testing.T) {
	got := renderTestLayout(t, `<article ht-apply class="card" aria-label="${ card.title }"><h2 ht-apply>${ card.title }</h2><p ht-apply>${ card.summary }</p></article>`, `{
  "card": {"title": "Arbitrary payload", "summary": "Rendered without page data."}
}`, false)
	want := `<article class="card" aria-label="Arbitrary payload"><h2>Arbitrary payload</h2><p>Rendered without page data.</p></article>`
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
	if strings.Contains(got, "<html") || strings.Contains(got, "generator") {
		t.Fatalf("fragment rendering added document output: %q", got)
	}
}

func TestRenderFragmentPreservesTopLevelNodes(t *testing.T) {
	got := renderTestLayout(t, `<h1 ht-apply>${ card.title }</h1><p ht-apply>${ card.summary }</p>`, `{
  "card": {"title": "First", "summary": "Second"}
}`, false)
	want := `<h1>First</h1><p>Second</p>`
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
}

func TestRenderFragmentHonorsTidyMode(t *testing.T) {
	layout := `<p ht-apply>${ card.title }</p>`
	data := `{"card": {"title": "Tidy"}}`
	if got, want := renderTestLayout(t, layout, data, false), `<p>Tidy</p>`; got != want {
		t.Fatalf("tidy Render() = %q, want %q", got, want)
	}
	if got, want := renderTestLayout(t, layout, data, true), `<p ht-apply="">Tidy</p>`; got != want {
		t.Fatalf("untidy Render() = %q, want %q", got, want)
	}
}

func TestRenderDocumentKeepsBuiltins(t *testing.T) {
	got := renderTestLayout(t, "<!doctype html><!-- leading --><html><head></head><body><main ht-apply>${ page.title }</main></body></html>", `{
  "page": {"title": "Document", "refresh": "", "canonical_url": "https://example.test/document"}
}`, false)
	for _, want := range []string{`<!DOCTYPE html>`, `<html><head>`, `name="generator"`, `<main>Document</main>`} {
		if !strings.Contains(got, want) {
			t.Fatalf("document Render() = %q, want %q", got, want)
		}
	}
}

func TestRenderFragmentSkipsRootOnlyDirectives(t *testing.T) {
	got := renderTestLayout(t, `<main ht-pipe='from ".component"'><span class="component">source</span></main><notice ht-block>fallback</notice>`, `{}`, false)
	for _, want := range []string{`ht-pipe="from &#34;.component&#34;"`, `<span class="component">source</span>`, `ht-block=""`} {
		if !strings.Contains(got, want) {
			t.Fatalf("fragment Render() = %q, want %q", got, want)
		}
	}
}
