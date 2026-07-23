package directives

import (
	"fmt"
	"io"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var HyperParamsAttributeStrings = []string{"data-ht-param", "ht-param"}

type ParamDirective struct{}

var _ Directive = ParamDirective{} // make the compiler check Directive interface conformance

func (d ParamDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-param"}
	if !strict {
		attributes = append(attributes, "ht-param")
	}
	return attributes
}

func (d ParamDirective) Selectors(strict bool) []string {
	selectors := []string{"[data-ht-param]:not([data-ht-template] [data-ht-param])"}
	if strict {
		return selectors
	}
	selectors[0] += ":not([ht-template] [data-ht-param])"
	return append(selectors, "[ht-param]:not([ht-template] [ht-param]):not([data-ht-template] [ht-param])")
}

func (d ParamDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (d ParamDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if document == nil {
		return nil
	}
	var fragment []*html.Node
	var fragmentDiv *html.Node = &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	var paramString string = htdocument.GetAttribute(element, HyperParamsAttributeStrings...).Val
	htdocument.OuterText(element, htdocument.GetAttribute(element, "data-default", "default").Val)
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
