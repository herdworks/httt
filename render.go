package httt

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/herdworks/httt/directives"
	"github.com/herdworks/httt/document"
	"github.com/herdworks/httt/theme"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func Render(dataPath string, layoutPath string, themePath string, untidy bool, markdownConfig *HyperMarkConfig, w io.Writer) ([]byte, error) {
	if w == nil {
		w = os.Stdout
	}

	var data TemplateData
	var layout *html.Node
	var t *theme.Theme
	var doc *document.Document
	var err error

	if data, err = ParseTemplateDataFile(dataPath, markdownConfig); err != nil {
		return nil, err
	}

	t = &theme.Theme{URI: &url.URL{Path: themePath}}
	if layout, err = renderLayout(t, layoutPath); err != nil {
		return nil, err
	}

	doc = &document.Document{
		Theme:          t,
		Layout:         layout,
		TidyMode:       !untidy,
		MarkdownConfig: markdownConfig,
	}
	if _, err = directives.Process(doc, &data, nil, w); err != nil {
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
