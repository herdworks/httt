package directives

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

// The Directive interface
type Directive interface {
	Attributes(strict bool) []string
	Selectors(strict bool) []string
	Elements(document *htdocument.Document) []*html.Node
	NextElements(document *htdocument.Document) []*html.Node
	Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) error
}

// DirectiveExpression is a directive with a parseable attribute expression
// syntax. Validation remains explicit so callers can control when it occurs.
type DirectiveExpression interface {
	Directive
	ParseExpression(string) ([]Token, error)
	ValidateExpression([]Token) error
}

// IncrementalDirectiveExpression optionally parses one delimiter-bounded
// expression segment at a time. ParseNextExpression is lexical only; callers
// explicitly validate the returned token sequence.
type IncrementalDirectiveExpression interface {
	DirectiveExpression
	ParseNextExpression(string) (remainder string, tokens []Token, err error)
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
		ConditionalDirective{},  // ht-if
		DebugDirective{},        // ht-debug
		PipeDirective{},         // ht-pipe
		EachDirective{},         // ht-each
		ApplyDirective{},        // ht-apply
		AttributeMapDirective{}, // ht-attrs
		BlockDirective{},        // ht-block
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

func defaultDirectiveSelectors(d Directive, strict bool) []string {
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
