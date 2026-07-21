package directives

import (
	"fmt"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var HyperParamsAttributeStrings = []string{"data-ht-param", "ht-param"}

func RenderParams(document *htdocument.Document, element *html.Node, data *htdata.TemplateData) (err error) {
	var fragment []*html.Node
	var fragmentDiv *html.Node = &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	var paramString string = htdocument.GetAttributeString(element, HyperParamsAttributeStrings...)
	htdocument.OuterText(element, htdocument.GetAttributeString(element, "data-default", "default"))
	for _, param := range ParseParameters(paramString, data) {
		if len(param.Values) == 0 {
			continue
		}
		if len(fmt.Sprintf("%v", param.Value())) == 0 {
			continue
		}
		switch param.Target {
		case "html":
			if fragment, err = htdocument.HTMLContent(fragmentDiv, param.Value()); err != nil {
				return err
			}
			htdocument.OuterHTML(element, fragment)
		case "markdown":
			if fragment, err = htdocument.MarkdownContent(fragmentDiv, document.MarkdownConfig, param.Value()); err != nil {
				return err
			}
			htdocument.OuterHTML(element, fragment)
		default:
			htdocument.OuterText(element, fmt.Sprintf("%v", param.Value()))
		}
	}
	return nil
}

func HyperParamsElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-param]:not([data-ht-template] [data-ht-param])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-param])"
	selectors = append(selectors, "[ht-param]:not([ht-template] [ht-param]):not([data-ht-template] [ht-param])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
