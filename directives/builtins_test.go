package directives

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	htdocument "github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/theme"
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
