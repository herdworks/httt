package directives

import (
	"fmt"
	"io"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

type DeprecatedDirective struct{}

var _ Directive = DeprecatedDirective{} // make the compiler check Directive interface conformance

func (DeprecatedDirective) Attributes(_ bool) []string {
	return []string{
		"data-ht-not", "ht-not",
		"data-ht-template", "ht-template",
		"data-ht-attr", "ht-attr",
		"data-ht-content", "ht-content",
		"data-ht-param", "ht-param",
		"data-ht-query", "ht-query",
	}
}

func (d DeprecatedDirective) Selectors(_ bool) []string {
	selectors := make([]string, 0, len(d.Attributes(false)))
	for _, attribute := range d.Attributes(false) {
		selectors = append(selectors, fmt.Sprintf("[%s]", attribute))
	}
	return selectors
}

func (d DeprecatedDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (DeprecatedDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (d DeprecatedDirective) Process(document *htdocument.Document, element *html.Node, _ *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if attributes := htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...); len(attributes) > 0 {
		return fmt.Errorf("the %q directive is no longer supported (see 'hyperctl theme migrate' for replacements);\n\n%s", attributes[0].Key, htdocument.ElementString(element))
	}
	return nil
}
