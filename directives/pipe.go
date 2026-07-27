package directives

import (
	"io"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperPipeAttributeStrings = []string{"data-ht-pipe", "ht-pipe"}

type PipeDirective struct{}

var _ Directive = PipeDirective{} // make the compiler check Directive interface conformance

func (d PipeDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-pipe"}
	if !strict {
		attributes = append(attributes, "ht-pipe")
	}
	return attributes
}

func (d PipeDirective) Selectors(strict bool) []string {
	selectors := []string{"[data-ht-pipe]"}
	if !strict {
		selectors = append(selectors, "[ht-pipe]")
	}
	return selectors
}

func (d PipeDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (PipeDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (d PipeDirective) Process(document *htdocument.Document, element *html.Node, _ *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if document == nil {
		return nil
	}
	var destination *html.Node
	paramString := htdocument.GetAttribute(element, HyperPipeAttributeStrings...).Val
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
