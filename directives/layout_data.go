package directives

import (
	"io"
	"strings"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

// LayoutDataDirective exposes layout:* metadata to later directive passes.
type LayoutDataDirective struct{}

var _ Directive = LayoutDataDirective{} // make the compiler check Directive interface conformance

func (LayoutDataDirective) Attributes(bool) []string { return []string{} }

func (LayoutDataDirective) Selectors(bool) []string { return []string{"head meta[name^='layout:']"} }

func (LayoutDataDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil || document.BlockMode || document.IsFragment() || document.Layout == nil {
		return nil
	}
	return htdocument.QueryAll(document.Layout, LayoutDataDirective{}.Selectors(document.StrictMode)...)
}

func (LayoutDataDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (LayoutDataDirective) Process(_ *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) error {
	layout, ok := (*data)["layout"].(htdata.TemplateData)
	if !ok {
		layout = htdata.TemplateData{}
		data.Set("layout", layout)
	}
	key := strings.TrimPrefix(htdocument.GetAttributeString(element, "name"), "layout:")
	layout.Set(key, htdocument.GetAttributeString(element, "content"))
	return nil
}
