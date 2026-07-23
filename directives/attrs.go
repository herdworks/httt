package directives

import (
	"fmt"
	"io"
	"strings"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperAttrAttributeStrings = []string{"data-ht-attr", "ht-attr"}
var HyperAttrsAttributeStrings = []string{"data-ht-attrs", "ht-attrs"}

type AttrsDirective struct{}

var _ Directive = AttrsDirective{} // make the compiler check Directive interface conformance

func (d AttrsDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-attrs", "data-ht-attr"}
	if !strict {
		attributes = append(attributes, "ht-attrs", "ht-attr")
	}
	return attributes
}

func (d AttrsDirective) Selectors(strict bool) []string {
	return defaultDirectiveSelectors(d, strict)
}

func (d AttrsDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (d AttrsDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if document == nil {
		return nil
	}
	paramString := htdocument.GetAttribute(element, HyperAttrsAttributeStrings...).Val
	for _, attribute := range htdocument.GetAttributes(element, HyperAttrAttributeStrings...) {
		paramString = strings.TrimSpace(paramString) + fmt.Sprintf("; %s", attribute.Val)
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
