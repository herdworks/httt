package directives

import (
	"fmt"
	"slices"
	"strings"
	texttemplate "text/template"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/variable"
	"golang.org/x/net/html"
)

var HyperApplyAttributeStrings = []string{"data-hyper-template", "hyper-template", "data-ht-apply", "ht-apply"}

func RenderVariables(document *htdocument.Document, element *html.Node, data *htdata.TemplateData) (err error) {
	if element == nil {
		return nil
	}
	if element.Type != html.ElementNode {
		return nil
	}

	funcs := document.Plugins.FuncMap()

	for index, attribute := range element.Attr {
		if slices.Contains(HyperApplyAttributeStrings, attribute.Key) {
			continue
		}
		syntax := variable.SyntaxDefault
		if attribute.Key == "style" {
			syntax = variable.SyntaxCSS
		}
		if !containsSyntaxMarker(attribute.Val, syntax) {
			continue
		}
		rendered, err := variable.RenderString(attribute.Val, data, syntax, funcs)
		if err != nil {
			return wrapVariableError(err, fmt.Sprintf("attribute %q", attribute.Key), element)
		}
		element.Attr[index].Val = rendered
	}

	if element.Data == "style" {
		for child := element.FirstChild; child != nil; child = child.NextSibling {
			if child.Type != html.TextNode {
				continue
			}
			if !containsSyntaxMarker(child.Data, variable.SyntaxCSS) {
				continue
			}
			rendered, err := variable.RenderString(child.Data, data, variable.SyntaxCSS, funcs)
			if err != nil {
				return wrapVariableError(err, "<style> text content", element)
			}
			child.Data = rendered
		}
	} else {
		nodeFuncs := variable.FuncMap{
			"text":     texttemplate.HTMLEscaper,
			"markdown": htdocument.MarkdownVariableFunc(element, document.MarkdownConfig),
		}
		var textChildren []*html.Node
		for child := element.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				textChildren = append(textChildren, child)
			}
		}
		for _, child := range textChildren {
			if !containsSyntaxMarker(child.Data, variable.SyntaxDefault) {
				continue
			}
			nodes, err := variable.RenderNodes(child.Data, data, element, funcs, nodeFuncs)
			if err != nil {
				return wrapVariableError(err, "element text content", element)
			}
			for _, node := range nodes {
				element.InsertBefore(node, child)
			}
			element.RemoveChild(child)
		}
	}

	if document.TidyMode {
		htdocument.RemoveAttributes(element, HyperApplyAttributeStrings...)
	}

	return nil
}

func HyperApplyElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-apply]:not([data-ht-template] [data-ht-apply], [data-ht-template][data-ht-apply])"}
	selectors = append(selectors, "[data-hyper-template]:not([data-ht-template] [data-hyper-template], [data-ht-template][data-hyper-template])")
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-apply], [ht-template][data-ht-apply])"
	selectors[1] += ":not([ht-template] [data-hyper-template], [ht-template][data-hyper-template])"
	selectors = append(selectors, "[ht-apply]:not([ht-template] [ht-apply], [ht-template][ht-apply]):not([data-ht-template] [ht-apply], [data-ht-template][ht-apply])")
	selectors = append(selectors, "[hyper-template]:not([ht-template] [hyper-template], [ht-template][hyper-template]):not([data-ht-template] [hyper-template], [data-ht-template][hyper-template])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}

func containsSyntaxMarker(source string, syntax variable.Syntax) bool {
	switch syntax {
	case variable.SyntaxCSS:
		return strings.Contains(source, "--ht-var(")
	default:
		return strings.Contains(source, "${")
	}
}

func wrapVariableError(err error, location string, element *html.Node) error {
	return fmt.Errorf(
		"failed rendering variables in %s:\n%s\n%w",
		location,
		htdocument.ElementString(element),
		err,
	)
}
