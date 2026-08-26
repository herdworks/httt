package directives

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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

// AttributeNames returns every recognized directive attribute name. When
// legacy is true, it also includes attributes from removed directives.
func AttributeNames(strict, legacy bool) []string {
	providers := []AttributeProvider{
		IncludeDirective{},
		DebugDirective{},
		ConditionalDirective{},
		EachDirective{},
		ApplyDirective{},
		AttributeMapDirective{},
		BlockDirective{},
		PipeDirective{},
		BaseDirective{},
	}
	if legacy {
		providers = append(providers, DeprecatedDirective{})
	}
	attributes := make([]string, 0)
	seen := map[string]bool{}
	add := func(attribute string) {
		if seen[attribute] {
			return
		}
		seen[attribute] = true
		attributes = append(attributes, attribute)
	}
	for _, provider := range providers {
		for _, attribute := range provider.Attributes(strict) {
			add(attribute)
		}
	}
	for _, attribute := range (EachDirective{}).offsetAttributes(strict) {
		add(attribute)
	}
	for _, attribute := range (EachDirective{}).limitAttributes(strict) {
		add(attribute)
	}
	return attributes
}

func defaultDirectiveSelectors(d AttributeProvider, strict bool) []string {
	attributes := d.Attributes(strict)
	selectors := make([]string, 0, len(attributes))
	boundaries := defaultBoundaryAttributes(strict)
	for _, attribute := range attributes {
		selectors = append(selectors, fmt.Sprintf(
			"[%s]%s%s",
			attribute,
			excludeBoundaryElements(boundaries, attribute),
			excludeBoundaryDescendants(boundaries, attribute),
		))
	}
	return selectors
}

func defaultBoundaryAttributes(strict bool) []string {
	attributes := []string{"data-ht-each", "data-ht-with"}
	if !strict {
		attributes = append(attributes, "ht-each", "ht-with")
	}
	return attributes
}

func baseBoundaryAttributes(strict bool) []string {
	attributes := []string{"data-ht-each", "data-ht-with", "data-ht-base"}
	if !strict {
		attributes = append(attributes, "ht-each", "ht-with", "ht-base")
	}
	return attributes
}

func excludeBoundaryElements(boundaries []string, attribute string) string {
	conditions := make([]string, 0, len(boundaries))
	for _, boundary := range boundaries {
		conditions = append(conditions, fmt.Sprintf("[%s][%s]", boundary, attribute))
	}
	return fmt.Sprintf(":not(%s)", strings.Join(conditions, ", "))
}

func excludeBoundaryDescendants(boundaries []string, attribute string) string {
	conditions := make([]string, 0, len(boundaries))
	for _, boundary := range boundaries {
		conditions = append(conditions, fmt.Sprintf("[%s] [%s]", boundary, attribute))
	}
	return fmt.Sprintf(":not(%s)", strings.Join(conditions, ", "))
}
