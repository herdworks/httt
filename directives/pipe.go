package directives

import (
	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperPipeAttributeStrings = []string{"data-ht-pipe", "ht-pipe"}

func RenderPipe(document *htdocument.Document, element *html.Node) (err error) {
	var destination *html.Node
	paramString := htdocument.GetAttributeString(element, HyperPipeAttributeStrings...)
	emptyData := htdata.TemplateData{}
	for _, param := range ParseParameters(paramString, &emptyData) {
		if destination = htdocument.Query(document.Layout, param.Keys...); destination == nil {
			continue
		}
		element.Parent.RemoveChild(element)
		switch param.Target {
		case "text", "css", "js", "javascript":
			htdocument.AppendText(destination, htdocument.TextContent(element, true))
		default:
			destination.AppendChild(element)
		}
	}
	if document.BlockMode {
		return nil
	}
	if document.TidyMode {
		htdocument.RemoveAttributes(element, HyperPipeAttributeStrings...)
	}
	return nil
}

func HyperPipeElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-pipe]:not([data-ht-template] [data-ht-pipe])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-pipe])"
	selectors = append(selectors, "[ht-pipe]:not([ht-template] [ht-pipe]):not([data-ht-template] [ht-pipe])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
