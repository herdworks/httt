package directives

import (
	"io"
	"os"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperForAttributeStrings = []string{"data-ht-for", "ht-for"}
var HyperTemplateAttributeStrings = []string{"data-ht-template", "ht-template"}
var HyperTemplateOffsetAttributeStrings = []string{"data-ht-offset", "ht-offset"}
var HyperTemplateLimitAttributeStrings = []string{"data-ht-limit", "ht-limit"}

func RenderTemplate(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

	var params Parameters
	var offset int = htdocument.GetAttributeInt(element, HyperTemplateOffsetAttributeStrings...)
	var limit int = htdocument.GetAttributeInt(element, HyperTemplateLimitAttributeStrings...)
	paramString := htdocument.GetAttributeString(element, HyperTemplateAttributeStrings...)
	if params = ParseParameters(paramString, data); len(params) == 0 {
		return nil
	}
	for index, value := range params[0].ValueSet() {
		index += 1
		if index <= offset || (limit > 0 && index > offset+limit) {
			continue
		}
		count := len(params[0].ValueSet())
		layoutFragment := htdocument.CloneElement(element)
		htdocument.RemoveAttributes(layoutFragment, HyperTemplateAttributeStrings...)

		var implicitApply bool
		if layoutFragment.Type == html.ElementNode && !htdocument.HasAttribute(layoutFragment, HyperApplyAttributeStrings...) {
			implicitApply = true
			htdocument.SetAttribute(layoutFragment, "data-ht-apply", "")
		}
		nestedData := &htdata.TemplateData{}
		nestedData.Set(params[0].Target, value)
		for _, p := range params[1:] {
			nestedData.Set(p.Target, p.Value())
		}
		nestedData.Set("ht.index", index)
		nestedData.Set("ht.count", count)
		nestedDoc := htdocument.Document{
			Layout:         layoutFragment,
			Theme:          document.Theme,
			TidyMode:       document.TidyMode,
			MarkdownConfig: document.MarkdownConfig,
			Plugins:        document.Plugins,
		}

		if _, err = process(&nestedDoc, nestedData, userData, w); err != nil {
			return err
		}
		if implicitApply {
			htdocument.RemoveAttributes(layoutFragment, "data-ht-apply")
		}
		if layoutFragment.Type == html.CommentNode {
			continue
		}
		if layoutFragment.Type == html.ElementNode {
			element.Parent.InsertBefore(layoutFragment, element)
		}
	}
	htdocument.SetAttribute(element, "hidden", "true")
	htdocument.RemoveAttributes(element, HyperTemplateOffsetAttributeStrings...)
	htdocument.RemoveAttributes(element, HyperTemplateLimitAttributeStrings...)
	if document.TidyMode {
		htdocument.RemoveElement(element)
	}
	return nil
}

func HyperTemplateElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-template]:not([data-ht-template] [data-ht-template])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-template])"
	selectors = append(selectors, "[ht-template]:not([ht-template] [ht-template]):not([data-ht-template] [ht-template])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}

func HyperLoopElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-for]:not([data-ht-for] [data-ht-for])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-for] [data-ht-for])"
	selectors = append(selectors, "[ht-for]:not([ht-for] [ht-for]):not([data-ht-for] [ht-template])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
