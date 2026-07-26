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

// Process resolves includes and renders the document's directive pipeline.
func Process(document *htdocument.Document, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) (htmlBytes []byte, err error) {
	if w == nil {
		w = os.Stdout
	}

	if document.Layout == nil {
		return nil, errors.New("template layout empty, nothing to do")
	}
	document.EnsureMarkdownConfig()

	// recursively resolve all include directives
	if err = ResolveIncludes(document); err != nil {
		return nil, err
	}

	// include builtin layout fragments
	if err = htdocument.IncludeBuiltins(document); err != nil {
		return nil, err
	}

	// Block fragments receive only caller-approved user data and block data.
	if !document.BlockMode {
		data.Set("layout", document.LayoutData())
	}

	// process directives
	directives := []Directive{
		DeprecatedDirective{},
		ConditionalDirective{},
		DebugDirective{},
		PipeDirective{},
		EachDirective{},
		ApplyDirective{},
		AttributeMapDirective{},
		BlockDirective{},
		BaseDirective{},
	}
	for _, directive := range directives {
		for _, element := range directive.Elements(document) {
			if err = directive.Process(document, element, data, userData, w); err != nil {
				return nil, err
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
