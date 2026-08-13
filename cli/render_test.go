package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/herdworks/clic"
)

func TestRenderCommandMatchesLearnGuide(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	dataPath := filepath.Join(root, "content", "index.md")
	layoutPath := filepath.Join(root, "layouts", "default.html")
	headerPath := filepath.Join(root, "fragments", "header.html")
	footerPath := filepath.Join(root, "fragments", "footer.html")

	if err := initializeTheme(root, "learn-hypertemplates", &bytes.Buffer{}); err != nil {
		t.Fatalf("initializeTheme() error = %v", err)
	}
	writeRenderGuideFile(t, dataPath, `---
title: Introduction to HTML templating
description: My first HyperTemplates page!
---

## Hello, world

This is my first HyperTemplates content!
`)

	writeRenderGuideFile(t, layoutPath, `<!doctype html>
<html lang='en-US'>
<head>
<meta charset='utf-8'>
<title ht-apply>${ title, "Learn HyperTemplates" }</title>
</head>
<body>
<main><article ht-apply>${ markdown(content) }</article></main>
</body>
</html>
`)
	lessonOne := renderLearnGuide(t, themePath, dataPath)
	assertRenderedContains(t, lessonOne,
		`<title>Introduction to HTML templating</title>`,
		`<article><h2 id="hello-world">Hello, world</h2>`,
		`<p>This is my first HyperTemplates content!</p>`,
	)

	writeRenderGuideFile(t, layoutPath, `<!doctype html>
<html lang='en-US'>
<head>
<meta charset='utf-8'>
<title ht-apply>${ title, "Learn HyperTemplates" }</title>
<meta ht-apply name='description' content='${ description }'>
</head>
<body>
<header ht-if='${ title }'><h1 ht-apply>${ title }</h1></header>
<main><article ht-apply>${ markdown(content) }</article></main>
<footer ht-apply ht-if='${ copyright }'>&copy; ${ copyright, "2024" } &bullet; Powered by HyperTemplates&trade;</footer>
</body>
</html>
`)
	lessonThreeWithoutCopyright := renderLearnGuide(t, themePath, dataPath)
	assertRenderedContains(t, lessonThreeWithoutCopyright,
		`<meta name="description" content="My first HyperTemplates page!"/>`,
		`<header><h1>Introduction to HTML templating</h1></header>`,
	)
	if strings.Contains(lessonThreeWithoutCopyright, "<footer") {
		t.Fatalf("conditional footer rendered without copyright: %q", lessonThreeWithoutCopyright)
	}

	writeRenderGuideFile(t, dataPath, `---
title: Introduction to HTML templating
description: My first HyperTemplates page!
copyright: 2025 Acme Inc
---

## Hello, world

This is my first HyperTemplates content!
`)
	writeRenderGuideFile(t, headerPath, `<header ht-if='${ title }'><nav><menu><a href='/'>Home</a><a href='/about/'>About</a></menu></nav><h1 ht-apply>${ title }</h1></header>`)
	writeRenderGuideFile(t, footerPath, `<footer ht-apply ht-if='${ copyright }'>&copy; ${ copyright, "2024" } &bullet; Powered by HyperTemplates&trade;</footer>`)
	writeRenderGuideFile(t, layoutPath, `<!doctype html>
<html lang='en-US'>
<head>
<meta charset='utf-8'>
<title ht-apply>${ title, "Learn HyperTemplates" }</title>
<meta ht-apply name='description' content='${ description }'>
</head>
<body>
<header ht-include='fragments/header.html' id='header'></header>
<main><article ht-apply>${ markdown(content) }</article></main>
<footer ht-include='fragments/footer' id='footer'></footer>
</body>
</html>
`)
	lessonFour := renderLearnGuide(t, themePath, dataPath)
	assertRenderedContains(t, lessonFour,
		`<header id="header">`,
		`<footer id="footer">© 2025 Acme Inc • Powered by HyperTemplates™</footer>`,
	)

	writeRenderGuideFile(t, headerPath, `<header ht-if='${ title }'><nav><menu><a ht-each='link in ${ nav }' href='${ link.href, link.url }'>${ link.title, link.label, link.text, "" }</a></menu></nav><h1 ht-apply>${ title, "Placeholder" }</h1></header>`)
	writeRenderGuideFile(t, dataPath, `---
title: Introduction to HTML templating
description: My first HyperTemplates page!
copyright: 2025 Acme Inc
nav:
  - label: Home
    href: /
  - title: About
    href: /about
  - label: Contact
    url: /contact
---

## Hello, world

This is my first HyperTemplates content!
`)
	lessonFive := renderLearnGuide(t, themePath, dataPath)
	assertRenderedContains(t, lessonFive,
		`<a href="/">Home</a>`,
		`<a href="/about">About</a>`,
		`<a href="/contact">Contact</a>`,
	)
}

func TestRenderCommandUsesConfiguredBlock(t *testing.T) {
	root := t.TempDir()
	themePath := filepath.Join(root, "theme.json")
	dataPath := filepath.Join(root, "data.json")
	blockPath := filepath.Join(root, "components", "notice.html")
	writeRenderGuideFile(t, themePath, `{"config":{"blocks_dir":"components"}}`)
	writeRenderGuideFile(t, dataPath, `{"message":"Block fixture"}`)
	writeRenderGuideFile(t, blockPath, `<notice ht-apply>${ message }</notice>`)

	var stdout bytes.Buffer
	result := CLI.Run(
		clic.Execution{
			Context: context.Background(),
			Stdout:  &stdout,
			Stderr:  &bytes.Buffer{},
		},
		clic.Arguments{
			"render",
			"--theme", themePath,
			"--data", dataPath,
			"--block", "notice",
		},
	)
	if result.Error != nil {
		t.Fatalf("httt render --block error = %v", result.Error)
	}
	if got, want := stdout.String(), "<notice>Block fixture</notice>\n"; got != want {
		t.Fatalf("httt render --block = %q, want %q", got, want)
	}
}

func TestRenderCommandRejectsMultipleResources(t *testing.T) {
	result := CLI.Run(
		clic.Execution{
			Context: context.Background(),
			Stdout:  &bytes.Buffer{},
			Stderr:  &bytes.Buffer{},
		},
		clic.Arguments{
			"render",
			"--theme", "theme.json",
			"--data", "data.json",
			"--layout", "default",
			"--block", "card",
		},
	)
	if result.Error == nil || !strings.Contains(result.Error.Error(), "exactly one of --layout, --fragment, or --block") {
		t.Fatalf("httt render with multiple resources error = %v", result.Error)
	}
}

func renderLearnGuide(t *testing.T, themePath, dataPath string) string {
	t.Helper()
	var stdout bytes.Buffer
	result := CLI.Run(
		clic.Execution{
			Context: context.Background(),
			Stdout:  &stdout,
			Stderr:  &bytes.Buffer{},
		},
		clic.Arguments{
			"render",
			"--theme", themePath,
			"--data", dataPath,
			"--layout", "default",
		},
	)
	if result.Error != nil {
		t.Fatalf("httt render error = %v", result.Error)
	}
	return stdout.String()
}

func writeRenderGuideFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertRenderedContains(t *testing.T, rendered string, values ...string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(rendered, value) {
			t.Fatalf("rendered output does not contain %q:\n%s", value, rendered)
		}
	}
}
