package directives

import (
	"bytes"
	"io"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/theme"
	"golang.org/x/net/html"
)

// BuiltinsDirective injects the default builtin fragments into a layout.
type BuiltinsDirective struct{}

var _ Directive = BuiltinsDirective{} // make the compiler check Directive interface conformance

func (BuiltinsDirective) Attributes(bool) []string { return []string{} }

func (BuiltinsDirective) Selectors(bool) []string { return nil }

func (BuiltinsDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil || document.Layout == nil || document.IsFragment() {
		return nil
	}
	return []*html.Node{document.Layout}
}

func (BuiltinsDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (BuiltinsDirective) Process(document *htdocument.Document, _ *html.Node, _ *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) error {
	if document == nil || document.BlockMode || document.IsFragment() {
		return nil
	}
	for _, slot := range theme.DefaultBuiltins() {
		if err := includeBuiltinSlot(document, slot); err != nil {
			return err
		}
	}
	return nil
}

func includeBuiltinSlot(document *htdocument.Document, slot theme.Builtin) error {
	fragmentBytes, found, err := resolveBuiltinFragment(document, slot.Fragment)
	if err != nil || !found || len(fragmentBytes) == 0 {
		return err
	}

	target := htdocument.Query(document.Layout, slot.Selector)
	if target == nil {
		return nil
	}

	parseParent := target
	switch slot.Mode {
	case "before", "after":
		if target.Parent == nil {
			return nil
		}
		parseParent = target.Parent
	}

	fragments, err := html.ParseFragment(bytes.NewReader(fragmentBytes), parseParent)
	if err != nil {
		return err
	}

	switch slot.Mode {
	case "prepend":
		anchor := target.FirstChild
		for _, node := range fragments {
			if anchor == nil {
				target.AppendChild(node)
			} else {
				target.InsertBefore(node, anchor)
			}
		}
	case "append":
		for _, node := range fragments {
			target.AppendChild(node)
		}
	case "before":
		for _, node := range fragments {
			target.Parent.InsertBefore(node, target)
		}
	case "after":
		anchor := target.NextSibling
		for _, node := range fragments {
			if anchor == nil {
				target.Parent.AppendChild(node)
			} else {
				target.Parent.InsertBefore(node, anchor)
			}
		}
	}
	return nil
}

func resolveBuiltinFragment(document *htdocument.Document, ref string) ([]byte, bool, error) {
	if document.Theme != nil {
		return document.Theme.ResourceBytes(ref)
	}
	return theme.ReadBuiltinResource(ref)
}
