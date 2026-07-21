package directives

import (
	"fmt"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperContentAttributeStrings = []string{"data-ht-content", "ht-content"}

func RenderContent(document *htdocument.Document, element *html.Node, data *htdata.TemplateData) (err error) {
	paramString := htdocument.GetAttributeString(element, HyperContentAttributeStrings...)
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

func HyperContentElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-content]:not([data-ht-template] [data-ht-content])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-content])"
	selectors = append(selectors, "[ht-content]:not([ht-template] [ht-content]):not([data-ht-template] [ht-content])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
