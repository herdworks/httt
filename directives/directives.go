package directives

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

// The ExpressiveDirective interface is a specification for directives with a
// defined grammar.
type ExpressiveDirective interface {
	GrammarProvider
	Directive
}

// The Directive interface is a specification for element processing directives.
type Directive interface {
	ElementProcessor
}

type ElementProcessor interface {
	ElementProvider
	Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) error
}

type ElementProvider interface {
	AttributeProvider
	Elements(document *htdocument.Document) []*html.Node
	NextElements(document *htdocument.Document) []*html.Node
}

type AttributeProvider interface {
	Attributes(strict bool) []string
	Selectors(strict bool) []string
}

type GrammarProvider interface {
	ParseExpression(string) ([]Token, error)
	ValidateExpression(Tokens) error
}

// Process renders the document's directive pipeline.
func Process(document *htdocument.Document, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) (htmlBytes []byte, err error) {
	if w == nil {
		w = os.Stdout
	}

	if document.Layout == nil {
		return nil, errors.New("template layout empty, nothing to do")
	}
	document.EnsureMarkdownConfig()

	// templating directives
	directives := []Directive{
		IncludeDirective{},      // ht-include
		BuiltinsDirective{},     //
		LayoutDataDirective{},   //
		DeprecatedDirective{},   //
		DebugDirective{},        // ht-debug
		ConditionalDirective{},  // ht-if
		EachDirective{},         // ht-each
		ApplyDirective{},        // ht-apply
		AttributeMapDirective{}, // ht-attrs
		BlockDirective{},        // ht-block
		PipeDirective{},         // ht-pipe
		BaseDirective{},         // ht-base
	}

	// Execute directives in declaration order.
	//
	// NOTE: Directives observe the DOM produced by preceding directives (if any).
	for _, directive := range directives {
		// Process each element in the initial directive.Elements() snapshot.
		//
		// Then optionally process elements returned by directive.NextElements().
		//
		// NOTE: Single-pass directives should return no elements from directive.NextElements().
		for elements := directive.Elements(document); len(elements) > 0; elements = directive.NextElements(document) {
			for _, element := range elements {
				if err = directive.Process(document, element, data, userData, w); err != nil {
					return nil, err
				}
			}
		}
	}

	return document.HTML(document.Layout), nil
}

func defaultDirectiveSelectors(d AttributeProvider, strict bool) []string {
	strictAttributes := d.Attributes(true)
	var selectors []string
	for _, attribute := range strictAttributes {
		selector := fmt.Sprintf("[%[1]s]:not([data-ht-each] [%[1]s], [data-ht-each][%[1]s])", attribute)
		if !strict {
			selector += fmt.Sprintf(":not([ht-each] [%[1]s], [ht-each][%[1]s])", attribute)
		}
		selectors = append(selectors, selector)
	}
	if !strict {
		attributes := slices.Clone(d.Attributes(false))
		attributes = slices.DeleteFunc(attributes, func(s string) bool { return slices.Contains(strictAttributes, s) })
		for _, attribute := range attributes {
			selector := fmt.Sprintf("[%[1]s]:not([ht-each] [%[1]s], [ht-each][%[1]s]):not([data-ht-each] [%[1]s], [data-ht-each][%[1]s])", attribute)
			selectors = append(selectors, selector)
		}
	}
	return selectors
}
