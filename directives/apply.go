package directives

import (
	"fmt"
	"io"
	"slices"
	"strings"
	texttemplate "text/template"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/variable"
	"golang.org/x/net/html"
)

type ApplyDirective struct{}

var _ Directive = ApplyDirective{} // make the compiler check Directive interface conformance

func (d ApplyDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-apply", "data-hyper-template"}
	if !strict {
		attributes = append(attributes, "ht-apply", "hyper-template")
	}
	return attributes
}

func (d ApplyDirective) Selectors(strict bool) []string {
	return defaultDirectiveSelectors(d, strict)
}

func (ApplyDirective) reservedAttributeNames() []string {
	return []string{
		"data-ht-attrs", "ht-attrs",
		"data-ht-base", "ht-base",
	}
}

func (d ApplyDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (d ApplyDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if document == nil {
		return nil
	}
	if element == nil {
		return nil
	}
	if element.Type != html.ElementNode {
		return nil
	}

	funcs := document.Plugins.FuncMap()

	for index, attribute := range element.Attr {
		if slices.Contains(d.Attributes(document.StrictMode), attribute.Key) || slices.Contains(d.reservedAttributeNames(), attribute.Key) {
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
		htdocument.RemoveAttributes(element, d.Attributes(false)...)
	}

	return nil
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
