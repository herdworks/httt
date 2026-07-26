package directives

import (
	"fmt"
	"io"
	"maps"
	"os"
	"reflect"
	"regexp"

	"golang.org/x/net/html"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	htvariable "github.com/herdworks/hypertemplates-go/variable"
)

// EachDirective expands one element for every item in an explicit typed source.
type EachDirective struct{}

var eachGrammar = MustCompileExpressionGrammar(ExpressionGrammar{Operators: []string{"in"}})

var _ Directive = EachDirective{}           // make the compiler check Directive interface conformance
var _ DirectiveExpression = EachDirective{} // make the compiler check DirectiveExpression interface conformance

var bindingSyntax = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (EachDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-each"}
	if !strict {
		attributes = append(attributes, "ht-each")
	}
	return attributes
}

func (d EachDirective) offsetAttributes(strict bool) []string {
	attributes := []string{"data-ht-offset"}
	if !strict {
		attributes = append(attributes, "ht-offset")
	}
	return attributes
}

func (d EachDirective) limitAttributes(strict bool) []string {
	attributes := []string{"data-ht-limit"}
	if !strict {
		attributes = append(attributes, "ht-limit")
	}
	return attributes
}

func (d EachDirective) allAttributes(strict bool) []string {
	attributes := d.Attributes(strict)
	attributes = append(attributes, d.offsetAttributes(strict)...)
	attributes = append(attributes, d.limitAttributes(strict)...)
	return attributes
}

func (d EachDirective) Selectors(strict bool) []string {
	if strict {
		return []string{"[data-ht-each]:not([data-ht-each] [data-ht-each])"}
	}
	return []string{
		"[data-ht-each]:not([data-ht-each] [data-ht-each]):not([ht-each] [data-ht-each])",
		"[ht-each]:not([ht-each] [ht-each]):not([data-ht-each] [ht-each])",
	}
}

func (d EachDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (EachDirective) Grammar() CompiledExpressionGrammar {
	return eachGrammar
}

func (d EachDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) (err error) {
	if document == nil || element == nil {
		return nil
	}
	if w == nil {
		w = os.Stdout
	}
	var binding string
	var values []any
	attributes := htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...)
	offset := htdocument.GetAttributeInt(element, d.offsetAttributes(document.StrictMode)...)
	limit := htdocument.GetAttributeInt(element, d.limitAttributes(document.StrictMode)...)
	if binding, values, err = d.expressionParser(attributes[0].Val, data); err != nil {
		return fmt.Errorf("invalid %q directive on %s: %w", attributes[0].Key, htdocument.ElementString(element), err)
	}
	for index, value := range values {
		index += 1
		var count int
		var ok bool
		if count, ok = d.expressionWindow(index, len(values), offset, limit); !ok {
			continue
		}

		// clone the layout template for mutation
		var implicitApply bool
		layoutFragment := htdocument.CloneElement(element)
		htdocument.RemoveAttributes(layoutFragment, d.allAttributes(false)...) // remove all EachDirective attributes regardless of StrictMode
		if implicitApply = !htdocument.HasAttribute(layoutFragment, ApplyDirective{}.Attributes(document.StrictMode)...); implicitApply {
			htdocument.SetAttribute(layoutFragment, "data-ht-apply", "")
		}

		// Copy the context's top-level bindings so the iteration binding and
		// metadata remain local without duplicating the complete data graph.
		// Template data is read-only during rendering; values under unchanged
		// bindings are intentionally shared with the outer context.
		nested := maps.Clone(*data)
		if nested == nil {
			nested = htdata.TemplateData{}
		}
		nested[binding] = value
		nested["each"] = htdata.TemplateData{"index": index, "count": count, "total": len(values)}

		// process the fragment and insert it into the parent element
		nestedDoc := htdocument.Document{Layout: layoutFragment, Theme: document.Theme, TidyMode: document.TidyMode, MarkdownConfig: document.MarkdownConfig, Plugins: document.Plugins}
		if _, err := Process(&nestedDoc, &nested, userData, w); err != nil {
			return err
		}
		if implicitApply {
			htdocument.RemoveAttributes(layoutFragment, "data-ht-apply")
		}
		if layoutFragment.Type == html.ElementNode && element.Parent != nil {
			element.Parent.InsertBefore(layoutFragment, element)
		}
	}

	// clean up
	htdocument.SetAttribute(element, "hidden", "true")
	if document.TidyMode {
		htdocument.RemoveElement(element)
	}
	return nil
}

func (d EachDirective) expressionParser(expression string, data htvariable.Getter) (binding string, values []any, err error) {
	tokens, err := ParseDirectiveExpression(d, expression)
	if err != nil {
		return "", nil, err
	}
	binding = tokens[0].Raw
	values, err = d.expressionValues(tokens[2].Raw, data)
	if err != nil {
		return "", nil, err
	}
	return binding, values, nil
}

func (d EachDirective) ValidateExpression(tokens []Token) error {
	if len(tokens) != 3 {
		return fmt.Errorf("expected exactly three tokens: binding in ${ variable }")
	}
	if tokens[0].Kind != TokenLiteral {
		return fmt.Errorf("expected iteration binding as the first token")
	}
	if tokens[0].Quoted || !bindingSyntax.MatchString(tokens[0].Raw) || tokens[0].Raw == "each" {
		return fmt.Errorf("invalid iteration binding %q", tokens[0].Raw)
	}
	if tokens[1].Kind != TokenOperator || tokens[1].Raw != "in" {
		return fmt.Errorf("expected iteration operator as the second token")
	}
	if tokens[2].Kind != TokenVariable {
		return fmt.Errorf("expected template variable ${ ... } as the third token")
	}
	return nil
}

func (d EachDirective) expressionValues(variable string, data htvariable.Getter) ([]any, error) {
	value, err := htvariable.ResolveValue(variable, data)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Array && rv.Kind() != reflect.Slice {
		return []any{value}, nil
	}
	items := make([]any, rv.Len())
	for i := range items {
		items[i] = rv.Index(i).Interface()
	}
	return items, nil
}

func (d EachDirective) expressionWindow(index, total, offset, limit int) (count int, ok bool) {
	offset = max(0, offset)
	limit = max(0, limit)
	count = max(0, total-offset)
	if limit > 0 {
		count = min(count, limit)
	}
	if index <= offset || index > offset+count {
		return count, false
	}
	return count, true
}
