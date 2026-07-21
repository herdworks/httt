package document

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herdworks/hypertemplates-go/theme"
	"golang.org/x/net/html"
)

func setupBuiltinSlotTest(t *testing.T, layoutHTML string, themeFiles map[string]string) *Document {
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
	return &Document{
		Layout: doc,
		Theme:  &theme.Theme{URI: &url.URL{Path: themePath}},
	}
}

func TestBuiltinSlotsMultiNodeOrdering(t *testing.T) {
	doc := setupBuiltinSlotTest(t,
		"<!doctype html><html><head><title>X</title></head><body></body></html>",
		map[string]string{
			"fragments/builtin/head_start.html": `<meta name="a"><meta name="b"><meta name="c">`,
		},
	)
	if err := includeBuiltinSlots(doc, []theme.Builtin{
		{Name: "head_start", Selector: "head", Mode: "prepend", Fragment: "fragments/builtin/head_start"},
	}); err != nil {
		t.Fatalf("includeBuiltinSlots: %v", err)
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
	doc := setupBuiltinSlotTest(t, "<!doctype html><html><head></head><body><div>foo</div></body></html>", nil)
	if err := includeBuiltinSlots(doc, []theme.Builtin{
		{Name: "missing", Selector: "section.does-not-exist", Mode: "prepend", Fragment: "fragments/builtin/head_start"},
	}); err != nil {
		t.Fatalf("includeBuiltinSlots: %v", err)
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
	doc := setupBuiltinSlotTest(t,
		`<!doctype html><html><head></head><body><div id="anchor"><span>middle</span></div></body></html>`,
		map[string]string{
			"fragments/builtin/p.html": `<p>P</p>`,
			"fragments/builtin/a.html": `<p>A</p>`,
			"fragments/builtin/b.html": `<p>B</p>`,
			"fragments/builtin/f.html": `<p>F</p>`,
		},
	)
	if err := includeBuiltinSlots(doc, []theme.Builtin{
		{Name: "p", Selector: "div#anchor", Mode: "prepend", Fragment: "fragments/builtin/p"},
		{Name: "a", Selector: "div#anchor", Mode: "append", Fragment: "fragments/builtin/a"},
		{Name: "b", Selector: "div#anchor", Mode: "before", Fragment: "fragments/builtin/b"},
		{Name: "f", Selector: "div#anchor", Mode: "after", Fragment: "fragments/builtin/f"},
	}); err != nil {
		t.Fatalf("includeBuiltinSlots: %v", err)
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
