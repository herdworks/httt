package httt

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/herdworks/httt/data"
	"github.com/herdworks/httt/directives"
	"github.com/herdworks/httt/document"
	"github.com/herdworks/httt/parse"
	"github.com/herdworks/httt/theme"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func Render(dataPath string, layoutPath string, themePath string, untidy bool, markdownConfig *HyperMarkConfig, w io.Writer) ([]byte, error) {
	if w == nil {
		w = os.Stdout
	}

	data, err := ParseTemplateDataFile(dataPath, markdownConfig)
	if err != nil {
		return nil, err
	}
	t := &theme.Theme{URI: &url.URL{Path: themePath}}
	layout, err := renderLayout(t, layoutPath)
	if err != nil {
		return nil, err
	}
	doc := &document.Document{Theme: t, Layout: layout, TidyMode: !untidy, MarkdownConfig: markdownConfig}
	if _, err = directives.Process(doc, &data, nil, w); err != nil {
		return nil, err
	}
	if doc.IsFragment() {
		return document.HTML(document.ChildNodes(doc.Layout)...), nil
	}
	return document.HTML(doc.Layout), nil
}

// RenderTheme renders a configured theme layout or fragment against template data.
// When binding is non-empty, source data is installed at that top-level key.
func RenderTheme(dataPath, themePath, layoutRef, fragmentRef, binding string, untidy bool, w io.Writer) ([]byte, error) {
	return renderTheme(dataPath, themePath, layoutRef, fragmentRef, "", binding, untidy, w)
}

// RenderThemeBlock renders a configured theme block against template data.
// When binding is non-empty, source data is installed at that top-level key.
func RenderThemeBlock(dataPath, themePath, blockRef, binding string, untidy bool, w io.Writer) ([]byte, error) {
	return renderTheme(dataPath, themePath, "", "", blockRef, binding, untidy, w)
}

func renderTheme(dataPath, themePath, layoutRef, fragmentRef, blockRef, binding string, untidy bool, w io.Writer) ([]byte, error) {
	resourceRefs := 0
	for _, ref := range []string{layoutRef, fragmentRef, blockRef} {
		if ref != "" {
			resourceRefs++
		}
	}
	if resourceRefs != 1 {
		return nil, fmt.Errorf("render requires exactly one resource reference")
	}
	raw, err := parse.ReadFile(dataPath)
	if err != nil {
		return nil, err
	}
	t := &theme.Theme{URI: &url.URL{Path: themePath}}
	if err = t.Load(io.Discard); err != nil {
		return nil, err
	}
	t.ResolveConfig()
	if err = t.Validate(); err != nil {
		return nil, err
	}
	if err = t.DiscoverPlugins(); err != nil {
		return nil, err
	}
	if binding != "" {
		raw = data.TemplateData{
			binding: raw,
		}
	}
	ref := ""
	switch {
	case layoutRef != "":
		ref = layoutRef
		ref = themeResourceRef(ref, t.LayoutsDirName())
	case fragmentRef != "":
		ref = fragmentRef
		ref = themeResourceRef(fragmentRef, t.FragmentsDirName())
	default:
		ref = blockRef
		ref = themeResourceRef(blockRef, t.BlocksDirName())
	}
	layout, err := renderLayout(t, ref)
	if err != nil {
		return nil, err
	}
	doc := &document.Document{Theme: t, Layout: layout, TidyMode: !untidy, MarkdownConfig: &DefaultHyperMarkConfig, Plugins: t.Plugins}
	if _, err = directives.Process(doc, &raw, nil, w); err != nil {
		return nil, err
	}
	if doc.IsFragment() {
		return document.HTML(document.ChildNodes(doc.Layout)...), nil
	}
	return document.HTML(doc.Layout), nil
}

// renderLayout parses an explicit HTML document as a DocumentNode and every
// other resource as a fragment. html.Parse always synthesizes a DocumentNode,
// so classify the raw resource before parsing it.
func renderLayout(t *theme.Theme, layoutPath string) (*html.Node, error) {
	resource, found, err := t.ResourceBytes(layoutPath)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("template resource %q not found", t.ResourcePath(layoutPath))
	}
	if hasHTMLRoot(resource) {
		return html.Parse(bytes.NewReader(resource))
	}
	container := &html.Node{Type: html.ElementNode, Data: atom.Div.String(), DataAtom: atom.Div}
	fragments, err := html.ParseFragment(bytes.NewReader(resource), container)
	if err != nil {
		return nil, err
	}
	for _, fragment := range fragments {
		container.AppendChild(fragment)
	}
	return container, nil
}

func themeResourceRef(ref, directory string) string {
	ref = filepath.ToSlash(ref)
	directory = filepath.ToSlash(directory)
	if strings.HasPrefix(ref, directory+"/") {
		return ref
	}
	return filepath.ToSlash(filepath.Join(directory, ref))
}

func hasHTMLRoot(source []byte) bool {
	tokenizer := html.NewTokenizer(bytes.NewReader(source))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return false
		case html.TextToken:
			if strings.TrimSpace(string(tokenizer.Raw())) != "" {
				return false
			}
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			return strings.EqualFold(string(name), atom.Html.String())
		case html.EndTagToken, html.SelfClosingTagToken:
			return false
		}
	}
}
