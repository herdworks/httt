package directives

import (
	"fmt"
	"strings"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperAttrAttributeStrings = []string{"data-ht-attr", "ht-attr"}
var HyperAttrsAttributeStrings = []string{"data-ht-attrs", "ht-attrs"}

func RenderAttributes(document *htdocument.Document, element *html.Node, data *htdata.TemplateData) (err error) {
	paramString := htdocument.GetAttributeString(element, HyperAttrsAttributeStrings...)
	for _, param := range htdocument.GetAttributeStrings(element, HyperAttrAttributeStrings...) {
		paramString = strings.TrimSpace(paramString) + fmt.Sprintf("; %s", param)
	}
	for _, param := range ParseParameters(paramString, data) {
		rawValue := param.Value()
		if rawValue == nil {
			continue
		}
		switch value := rawValue.(type) {
		case htdata.TemplateData:
			for k, v := range value {
				htdocument.SetAttribute(element, k, fmt.Sprintf("%v", v))
			}
		case map[string]any:
			for k, v := range value {
				htdocument.SetAttribute(element, k, fmt.Sprintf("%v", v))
			}
		default:
			htdocument.SetAttribute(element, param.Target, fmt.Sprintf("%v", value))
		}
		if document.TidyMode {
			htdocument.RemoveAttributes(element, HyperAttrsAttributeStrings...)
			htdocument.RemoveAttributes(element, HyperAttrAttributeStrings...)
		}
	}
	return nil
}

func HyperAttrsElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-attrs]:not([data-ht-template] [data-ht-attrs], [data-ht-template][data-ht-attrs])"}
	selectors = append(selectors, "[data-ht-attr]:not([data-ht-template] [data-ht-attr], [data-ht-template][data-ht-attr])")
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-attrs], [ht-template][data-ht-attrs])"
	selectors[1] += ":not([ht-template] [data-ht-attr], [ht-template][data-ht-attr])"
	selectors = append(selectors, "[ht-attrs]:not([ht-template] [ht-attrs], [ht-template][ht-attrs]):not([data-ht-template] [ht-attrs], [data-ht-template][ht-attrs])")
	selectors = append(selectors, "[ht-attr]:not([ht-template] [ht-attr], [ht-template][ht-attr]):not([data-ht-template] [ht-attr], [data-ht-template][ht-attr])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
