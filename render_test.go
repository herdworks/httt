package httt

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
	if err := os.WriteFile(layoutPath, []byte(layout), 0o644); err != nil {
		t.Fatalf("write layout: %v", err)
	}
	if err := os.WriteFile(dataPath, []byte(data), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}
	rendered, err := Render(dataPath, "layout.html", filepath.Join(root, "theme.json"), untidy, &DefaultHyperMarkConfig, nil)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return string(rendered)
}

func TestRenderFragmentWithArbitraryData(t *testing.T) {
	got := renderTestLayout(t, `<article ht-apply class="card" aria-label="${ card.title }"><h2 ht-apply>${ card.title }</h2><p ht-apply>${ card.summary }</p></article>`, `{"card":{"title":"Arbitrary payload","summary":"Rendered without page data."}}`, false)
	want := `<article class="card" aria-label="Arbitrary payload"><h2>Arbitrary payload</h2><p>Rendered without page data.</p></article>`
	if got != want {
		t.Fatalf("Render() = %q, want %q", got, want)
	}
	if strings.Contains(got, "<html") || strings.Contains(got, "generator") {
		t.Fatalf("fragment rendering added document output: %q", got)
	}
}

func TestRenderFragmentPreservesTopLevelNodes(t *testing.T) {
	got := renderTestLayout(t, `<h1 ht-apply>${ card.title }</h1><p ht-apply>${ card.summary }</p>`, `{"card":{"title":"First","summary":"Second"}}`, false)
	if got != `<h1>First</h1><p>Second</p>` {
		t.Fatalf("Render() = %q", got)
	}
}

func TestRenderFragmentHonorsTidyMode(t *testing.T) {
	layout := `<p ht-apply>${ card.title }</p>`
	data := `{"card":{"title":"Tidy"}}`
	if got := renderTestLayout(t, layout, data, false); got != `<p>Tidy</p>` {
		t.Fatalf("tidy Render() = %q", got)
	}
	if got := renderTestLayout(t, layout, data, true); got != `<p ht-apply="">Tidy</p>` {
		t.Fatalf("untidy Render() = %q", got)
	}
}

func TestRenderDocumentKeepsBuiltins(t *testing.T) {
	got := renderTestLayout(t, "<!doctype html><html><head></head><body><main ht-apply>${ page.title }</main></body></html>", `{"page":{"title":"Document","refresh":"","canonical_url":"https://example.test/document"}}`, false)
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

func TestRenderThemeUsesConfiguredFragmentDirectory(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	if err := os.WriteFile(themePath, []byte(`{"config":{"fragments_dir":"components"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "components", "card.html"), []byte(`<p ht-apply>${ card.title }</p>`), 0o644); err != nil {
		t.Fatal(err)
	}
	dataPath := filepath.Join(root, "data.json")
	if err := os.WriteFile(dataPath, []byte(`{"card":{"title":"Fixture"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := RenderTheme(dataPath, themePath, "", "card", "", false, nil)
	if err != nil {
		t.Fatalf("RenderTheme() error = %v", err)
	}
	if got, want := string(output), `<p>Fixture</p>`; got != want {
		t.Fatalf("RenderTheme() = %q, want %q", got, want)
	}
}

func TestRenderThemeBlockUsesConfiguredBlocksDirectory(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	if err := os.WriteFile(themePath, []byte(`{"config":{"blocks_dir":"components"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "components", "card.html"), []byte(`<card ht-apply>${ card.title }</card>`), 0o644); err != nil {
		t.Fatal(err)
	}
	dataPath := filepath.Join(root, "data.json")
	if err := os.WriteFile(dataPath, []byte(`{"card":{"title":"Fixture"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := RenderThemeBlock(dataPath, themePath, "card", "", false, nil)
	if err != nil {
		t.Fatalf("RenderThemeBlock() error = %v", err)
	}
	if got, want := string(output), `<card>Fixture</card>`; got != want {
		t.Fatalf("RenderThemeBlock() = %q, want %q", got, want)
	}
}

func TestRenderThemeUsesProvidedTopLevelData(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	if err := os.WriteFile(themePath, []byte(`{"data":{"label":"Theme data"},"config":{"fragments_dir":"components"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "components", "card.html"), []byte(`<p ht-apply>${ theme.label }</p>`), 0o644); err != nil {
		t.Fatal(err)
	}
	dataPath := filepath.Join(root, "data.json")
	if err := os.WriteFile(dataPath, []byte(`{"theme":{"label":"Provided theme"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := RenderTheme(dataPath, themePath, "", "card", "", false, nil)
	if err != nil {
		t.Fatalf("RenderTheme() error = %v", err)
	}
	if got, want := string(output), `<p>Provided theme</p>`; got != want {
		t.Fatalf("RenderTheme() = %q, want %q", got, want)
	}
}

func TestRenderThemeDoesNotInjectThemeConfigurationData(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	if err := os.WriteFile(themePath, []byte(`{"data":{"label":"Theme data"},"config":{"fragments_dir":"components"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "components", "card.html"), []byte(`<p ht-apply>${ theme.label }</p>`), 0o644); err != nil {
		t.Fatal(err)
	}
	dataPath := filepath.Join(root, "data.json")
	if err := os.WriteFile(dataPath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderTheme(dataPath, themePath, "", "card", "", false, nil); err == nil {
		t.Fatal("RenderTheme() injected theme.json data")
	}
}

func TestRenderThemeBindsOnlyProvidedData(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	if err := os.WriteFile(themePath, []byte(`{"data":{"label":"Theme data"},"config":{"fragments_dir":"components"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "components", "card.html"), []byte(`<p ht-apply>${ page.title }: ${ page.theme.label }</p>`), 0o644); err != nil {
		t.Fatal(err)
	}
	dataPath := filepath.Join(root, "data.json")
	if err := os.WriteFile(dataPath, []byte(`{"title":"Bound data","theme":{"label":"Ignored"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	output, err := RenderTheme(dataPath, themePath, "", "card", "page", false, nil)
	if err != nil {
		t.Fatalf("RenderTheme() error = %v", err)
	}
	if got, want := string(output), `<p>Bound data: Ignored</p>`; got != want {
		t.Fatalf("RenderTheme() = %q, want %q", got, want)
	}
}
