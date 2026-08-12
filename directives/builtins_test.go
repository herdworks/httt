package directives

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	htdocument "github.com/herdworks/httt/document"
	"github.com/herdworks/httt/theme"
	"golang.org/x/net/html"
)

// helper: set up an isolated theme rooted at a temp dir and parse the given
// layout HTML into a *Document ready for Process(). When themeFiles is
// non-nil, each entry creates a theme file at the given relative path.
func setupBuiltinTest(t *testing.T, layoutHTML string, themeFiles map[string]string) *htdocument.Document {
	t.Helper()
	dir := t.TempDir()
	themePath := filepath.Join(dir, "theme.json")
	if err := os.WriteFile(themePath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write theme.json: %v", err)
	}
	for relPath, content := range themeFiles {
		full := filepath.Join(dir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %q: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %q: %v", full, err)
		}
	}
	doc, err := html.Parse(bytes.NewReader([]byte(layoutHTML)))
	if err != nil {
		t.Fatalf("parse layout: %v", err)
	}
	return &htdocument.Document{
		Layout: doc,
		Theme:  &theme.Theme{URI: &url.URL{Path: themePath}},
	}
}

func processOrFatal(t *testing.T, document *htdocument.Document) string {
	t.Helper()
	out, err := Process(document, &TemplateData{}, &TemplateData{}, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	return string(out)
}

// withBuiltinsFS temporarily replaces BuiltinsFS for the duration of a test.
func withBuiltinsFS(t *testing.T, fsys fstest.MapFS) {
	t.Helper()
	saved := theme.BuiltinsFS
	theme.BuiltinsFS = fsys
	t.Cleanup(func() { theme.BuiltinsFS = saved })
}

// TestBuiltinsDefaultApplies — the embedded head_start fragment is injected
// into <head> before the existing children.
func TestBuiltinsDefaultApplies(t *testing.T) {
	tmpl := setupBuiltinTest(t, "<!doctype html><html><head><title>X</title></head><body></body></html>", nil)
	out := processOrFatal(t, tmpl)
	if !strings.Contains(out, "head_start.html begin") {
		t.Errorf("missing head_start begin marker in output:\n%s", out)
	}
	if !strings.Contains(out, `name="generator"`) {
		t.Errorf("missing generator meta from embedded head_start:\n%s", out)
	}
	if !strings.Contains(out, "<title>X</title>") {
		t.Errorf("original <title> lost:\n%s", out)
	}
	startIdx := strings.Index(out, "head_start.html begin")
	titleIdx := strings.Index(out, "<title>X</title>")
	if !(startIdx < titleIdx) {
		t.Errorf("expected head_start before title, got %d / %d:\n%s", startIdx, titleIdx, out)
	}
}

// TestBuiltinsThemeOverride — a theme file at the same relative path replaces
// the embedded default.
func TestBuiltinsThemeOverride(t *testing.T) {
	tmpl := setupBuiltinTest(t,
		"<!doctype html><html><head><title>X</title></head><body></body></html>",
		map[string]string{
			"fragments/builtin/head_start.html": `<meta name="x" content="y">`,
		},
	)
	out := processOrFatal(t, tmpl)
	if !strings.Contains(out, `<meta name="x" content="y"/>`) && !strings.Contains(out, `<meta name="x" content="y">`) {
		t.Errorf("expected override <meta> in output:\n%s", out)
	}
	if strings.Contains(out, "head_start.html begin") {
		t.Errorf("embedded default leaked despite override:\n%s", out)
	}
}

// TestBuiltinsEmptyDisables — empty theme file opts out (no fallback to embed).
func TestBuiltinsEmptyDisables(t *testing.T) {
	tmpl := setupBuiltinTest(t,
		"<!doctype html><html><head><title>X</title></head><body></body></html>",
		map[string]string{
			"fragments/builtin/head_start.html": "",
		},
	)
	out := processOrFatal(t, tmpl)
	if strings.Contains(out, "head_start.html begin") {
		t.Errorf("embedded default leaked despite empty-file opt-out:\n%s", out)
	}
	if strings.Contains(out, `name="generator"`) {
		t.Errorf("generator meta leaked despite empty-file opt-out:\n%s", out)
	}
}

// TestBuiltinsDirectiveEvaluation — value DSL inside a builtin fragment is
// resolved by the main pipeline (since builtins are injected before directive passes).
func TestBuiltinsDirectiveEvaluation(t *testing.T) {
	tmpl := setupBuiltinTest(t,
		`<!doctype html><html><head><title>X</title></head><body></body></html>`,
		map[string]string{
			"fragments/builtin/head_start.html": `<meta ht-apply name="generator" content="${ page.title }">`,
		},
	)
	data := &TemplateData{}
	data.Set("page.title", "Hello")
	out, err := Process(tmpl, data, &TemplateData{}, nil)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !strings.Contains(string(out), `content="Hello"`) {
		t.Errorf("expected resolved content=\"Hello\":\n%s", string(out))
	}
}

// TestBuiltinsHtIncludeFallback — `ht-include` resolves embedded refs even
// when the theme has no file, because the fallback layer applies to all
// resource lookups.
func TestBuiltinsHtIncludeFallback(t *testing.T) {
	tmpl := setupBuiltinTest(t,
		`<!doctype html><html><head></head><body><div ht-include="fragments/builtin/head_start"></div></body></html>`,
		nil,
	)
	out := processOrFatal(t, tmpl)
	if !strings.Contains(out, "head_start.html begin") {
		t.Errorf("ht-include did not resolve via embedded fallback:\n%s", out)
	}
}

func TestBuiltinsHtIncludeThemeOverride(t *testing.T) {
	withBuiltinsFS(t, fstest.MapFS{
		"builtins/fragments/override.html": &fstest.MapFile{Data: []byte(`<p>builtin fallback</p>`)},
	})
	tmpl := setupBuiltinTest(t,
		`<!doctype html><html><head></head><body><div ht-include="fragments/override"></div></body></html>`,
		map[string]string{"fragments/override.html": `<p>theme override</p>`},
	)
	out := processOrFatal(t, tmpl)
	if !strings.Contains(out, "theme override") {
		t.Errorf("ht-include did not resolve the theme resource:\n%s", out)
	}
	if strings.Contains(out, "builtin fallback") {
		t.Errorf("ht-include used the builtin despite a theme override:\n%s", out)
	}
}

func TestBuiltinsHtIncludeEmptyThemeOverride(t *testing.T) {
	withBuiltinsFS(t, fstest.MapFS{
		"builtins/fragments/empty.html": &fstest.MapFile{Data: []byte(`<p>builtin fallback</p>`)},
	})
	tmpl := setupBuiltinTest(t,
		`<!doctype html><html><head></head><body><div ht-include="fragments/empty"></div></body></html>`,
		map[string]string{"fragments/empty.html": ""},
	)
	out := processOrFatal(t, tmpl)
	if strings.Contains(out, "builtin fallback") {
		t.Errorf("ht-include used the builtin despite an empty theme override:\n%s", out)
	}
}

// TestBuiltinsFSSwap — replacing BuiltinsFS surfaces alternative content.
func TestBuiltinsFSSwap(t *testing.T) {
	withBuiltinsFS(t, fstest.MapFS{
		"builtins/fragments/builtin/head_start.html": &fstest.MapFile{Data: []byte(`<meta name="swapped">`)},
		"builtins/fragments/builtin/head_last.html":  &fstest.MapFile{Data: []byte(``)},
	})
	tmpl := setupBuiltinTest(t,
		`<!doctype html><html><head><title>X</title></head><body></body></html>`,
		nil,
	)
	out := processOrFatal(t, tmpl)
	if !strings.Contains(out, `name="swapped"`) {
		t.Errorf("expected swapped builtin content:\n%s", out)
	}
	if strings.Contains(out, "head_start.html begin") {
		t.Errorf("default-FS content leaked despite swap:\n%s", out)
	}
}

// TestBuiltinsBlockModeSkip — Templates in BlockMode skip builtin injection.
func TestBuiltinsBlockModeSkip(t *testing.T) {
	tmpl := setupBuiltinTest(t,
		`<!doctype html><html><head><title>X</title></head><body></body></html>`,
		nil,
	)
	tmpl.BlockMode = true
	out := processOrFatal(t, tmpl)
	if strings.Contains(out, "head_start.html begin") {
		t.Errorf("builtins ran in BlockMode:\n%s", out)
	}
}

func TestBuiltinSlotsMultiNodeOrdering(t *testing.T) {
	doc := setupBuiltinTest(t,
		"<!doctype html><html><head><title>X</title></head><body></body></html>",
		map[string]string{
			"fragments/builtin/head_start.html": `<meta name="a"><meta name="b"><meta name="c">`,
		},
	)
	if err := includeBuiltinSlot(doc, theme.Builtin{Name: "head_start", Selector: "head", Mode: "prepend", Fragment: "fragments/builtin/head_start"}); err != nil {
		t.Fatalf("includeBuiltinSlot: %v", err)
	}
	out := string(doc.HTML(doc.Layout))
	aIdx := strings.Index(out, `name="a"`)
	bIdx := strings.Index(out, `name="b"`)
	cIdx := strings.Index(out, `name="c"`)
	titleIdx := strings.Index(out, "<title>X</title>")
	if aIdx < 0 || bIdx < 0 || cIdx < 0 || titleIdx < 0 {
		t.Fatalf("missing nodes in output:\n%s", out)
	}
	if !(aIdx < bIdx && bIdx < cIdx && cIdx < titleIdx) {
		t.Errorf("expected order a < b < c < title, got %d / %d / %d / %d:\n%s", aIdx, bIdx, cIdx, titleIdx, out)
	}
}

func TestBuiltinSlotsNoSelectorMatch(t *testing.T) {
	doc := setupBuiltinTest(t, "<!doctype html><html><head></head><body><div>foo</div></body></html>", nil)
	if err := includeBuiltinSlot(doc, theme.Builtin{Name: "missing", Selector: "section.does-not-exist", Mode: "prepend", Fragment: "fragments/builtin/head_start"}); err != nil {
		t.Fatalf("includeBuiltinSlot: %v", err)
	}
	out := string(doc.HTML(doc.Layout))
	if !strings.Contains(out, "<div>foo</div>") {
		t.Errorf("output unexpectedly modified:\n%s", out)
	}
	if strings.Contains(out, "head_start.html") {
		t.Errorf("fragment injected despite no selector match:\n%s", out)
	}
}

func TestBuiltinSlotsAllFourModes(t *testing.T) {
	doc := setupBuiltinTest(t,
		`<!doctype html><html><head></head><body><div id="anchor"><span>middle</span></div></body></html>`,
		map[string]string{
			"fragments/builtin/p.html": `<p>P</p>`,
			"fragments/builtin/a.html": `<p>A</p>`,
			"fragments/builtin/b.html": `<p>B</p>`,
			"fragments/builtin/f.html": `<p>F</p>`,
		},
	)
	for _, slot := range []theme.Builtin{
		{Name: "p", Selector: "div#anchor", Mode: "prepend", Fragment: "fragments/builtin/p"},
		{Name: "a", Selector: "div#anchor", Mode: "append", Fragment: "fragments/builtin/a"},
		{Name: "b", Selector: "div#anchor", Mode: "before", Fragment: "fragments/builtin/b"},
		{Name: "f", Selector: "div#anchor", Mode: "after", Fragment: "fragments/builtin/f"},
	} {
		if err := includeBuiltinSlot(doc, slot); err != nil {
			t.Fatalf("includeBuiltinSlot(%q): %v", slot.Name, err)
		}
	}
	out := string(doc.HTML(doc.Layout))
	bIdx := strings.Index(out, "<p>B</p>")
	pIdx := strings.Index(out, "<p>P</p>")
	mIdx := strings.Index(out, "<span>middle</span>")
	aIdx := strings.Index(out, "<p>A</p>")
	fIdx := strings.Index(out, "<p>F</p>")
	if bIdx < 0 || pIdx < 0 || mIdx < 0 || aIdx < 0 || fIdx < 0 {
		t.Fatalf("missing one of B/P/middle/A/F:\n%s", out)
	}
	if !(bIdx < pIdx && pIdx < mIdx && mIdx < aIdx && aIdx < fIdx) {
		t.Errorf("expected order B < P < middle < A < F, got %d / %d / %d / %d / %d:\n%s", bIdx, pIdx, mIdx, aIdx, fIdx, out)
	}
}

func TestPipelineSetupDirectiveOrder(t *testing.T) {
	doc := setupBuiltinTest(t,
		`<!doctype html><html><head><meta ht-include="fragments/layout-meta.html"></head><body><p ht-apply>${ layout.source }</p></body></html>`,
		map[string]string{
			"fragments/layout-meta.html":        `<meta name="layout:source" content="included">`,
			"fragments/builtin/head_start.html": `<meta ht-apply name="builtin" content="${ layout.source }">`,
		},
	)
	out := processOrFatal(t, doc)
	if !strings.Contains(out, `name="builtin"`) || !strings.Contains(out, `content="included"`) || !strings.Contains(out, ">included</p>") {
		t.Fatalf("setup directives did not run include, builtins, then layout data: %s", out)
	}
}

func TestLayoutDataDirectiveSkipsBlockMode(t *testing.T) {
	doc := setupBuiltinTest(t,
		`<!doctype html><html><head><meta name="layout:source" content="document"></head><body></body></html>`,
		nil,
	)
	doc.BlockMode = true
	data := TemplateData{"layout": "caller"}
	if _, err := Process(doc, &data, nil, nil); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if got := data["layout"]; got != "caller" {
		t.Errorf("block render layout data = %#v, want caller value", got)
	}
}

func TestLayoutDataDirectiveSelectsHeadMetadataInLayoutsOnly(t *testing.T) {
	doc := setupBuiltinTest(t,
		`<!doctype html><html><head><meta name="layout:head" content="included"></head><body><meta name="layout:body" content="ignored"></body></html>`,
		nil,
	)
	data := TemplateData{}
	if _, err := Process(doc, &data, nil, nil); err != nil {
		t.Fatalf("Process: %v", err)
	}
	layout, ok := data["layout"].(TemplateData)
	if !ok || layout["head"] != "included" {
		t.Fatalf("layout data = %#v, want head metadata", data["layout"])
	}
	if _, found := layout["body"]; found {
		t.Fatalf("layout data unexpectedly includes body metadata: %#v", layout)
	}

	fragment := &htdocument.Document{Layout: doc.Layout.FirstChild}
	if elements := (LayoutDataDirective{}).Elements(fragment); len(elements) != 0 {
		t.Fatalf("fragment layout metadata elements = %d, want 0", len(elements))
	}
}
