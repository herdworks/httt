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

func (d DeprecatedDirective) Attributes(strict bool) []string {
	directives := []string{"data-ht-not"}
	if !strict {
		directives = append(directives, "ht-not")
	}
	return directives
}

func (d DeprecatedDirective) Selectors(strict bool) []string {
	selectors := []string{}
	for _, directive := range d.Attributes(strict) {
		selectors = append(selectors, fmt.Sprintf("[%s]", directive))
	}
	return selectors
}

func (d DeprecatedDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (d DeprecatedDirective) Process(document *htdocument.Document, element *html.Node, _ *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if attributes := htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...); len(attributes) > 0 {
		return fmt.Errorf("the %q directive is no longer supported (see 'hyperctl theme migrate' for replacements);\n\n%s", attributes[0].Key, htdocument.ElementString(element))
	}
	return nil
}
