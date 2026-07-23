package directives

import (
	"fmt"
	"io"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperContentAttributeStrings = []string{"data-ht-content", "ht-content"}

type ContentDirective struct{}

var _ Directive = ContentDirective{} // make the compiler check Directive interface conformance

func (d ContentDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-content"}
	if !strict {
		attributes = append(attributes, "ht-content")
	}
	return attributes
}

func (d ContentDirective) Selectors(strict bool) []string {
	selectors := []string{"[data-ht-content]:not([data-ht-template] [data-ht-content])"}
	if strict {
		return selectors
	}
	selectors[0] += ":not([ht-template] [data-ht-content])"
	return append(selectors, "[ht-content]:not([ht-template] [ht-content]):not([data-ht-template] [ht-content])")
}

func (d ContentDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (d ContentDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if document == nil {
		return nil
	}
	paramString := htdocument.GetAttribute(element, HyperContentAttributeStrings...).Val
	for _, param := range ParseParameters(paramString, data) {
		if len(param.Values) == 0 {
			continue
		}
		switch param.Target {
		case "html":
			fragment, err := htdocument.HTMLContent(element, param.Value())
			if err != nil {
				return err
			}
			htdocument.InnerHTML(element, fragment)
		case "markdown":
			fragment, err := htdocument.MarkdownContent(element, document.MarkdownConfig, param.Value())
			if err != nil {
				return err
			}
			htdocument.InnerHTML(element, fragment)
		default:
			htdocument.InnerText(element, fmt.Sprintf("%v", param.Value()))
		}
	}
	if document.TidyMode {
		htdocument.RemoveAttributes(element, HyperContentAttributeStrings...)
	}
	return nil
}
