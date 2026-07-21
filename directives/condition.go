package directives

import (
	"io"
	"os"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperConditionalAttributeStrings = []string{"data-ht-if", "ht-if", "data-ht-not", "ht-not"}
var HyperIfAttributeStrings = []string{"data-ht-if", "ht-if"}
var HyperNotAttributeStrings = []string{"data-ht-not", "ht-not"}

func EvaluateElement(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, inclusive bool, w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

	paramString := htdocument.GetAttributeString(element, HyperConditionalAttributeStrings...)
	for _, param := range ParseParameters(paramString, data) {
		if param.Evaluate(w) == inclusive {
			htdocument.RemoveAttributes(element, "hidden")
		} else {
			htdocument.SetAttribute(element, "hidden", "true")
			if document.TidyMode {
				htdocument.RemoveElement(element)
			}
		}
	}
	if document.TidyMode {
		htdocument.RemoveAttributes(element, HyperConditionalAttributeStrings...)
	}
	return nil
}

func HyperIfElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-if]:not([data-ht-template] [data-ht-if], [data-ht-template][data-ht-if])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-if], [ht-template][data-ht-if])"
	selectors = append(selectors, "[ht-if]:not([ht-template] [ht-if], [ht-template][ht-if]):not([data-ht-template] [ht-if], [data-ht-template][ht-if])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}

func HyperNotElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-not]:not([data-ht-template] [data-ht-not], [data-ht-template][data-ht-not])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-not], [ht-template][data-ht-not])"
	selectors = append(selectors, "[ht-not]:not([ht-template] [ht-not], [ht-template][ht-not]):not([data-ht-template] [ht-not], [data-ht-template][ht-not])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
