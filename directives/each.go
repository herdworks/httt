package directives

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"

	"golang.org/x/net/html"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	htvariable "github.com/herdworks/httt/variable"
)

// EachDirective expands one element for every item in an explicit typed source.
type EachDirective struct{}

type EachDirectiveBindMode int

const (
	BindNone EachDirectiveBindMode = iota
	BindEach
	BindWith
	BindError
)

var _ Directive = EachDirective{}           // make the compiler check Directive interface conformance
var _ ExpressiveDirective = EachDirective{} // make the compiler check ExpressiveDirective interface conformance

var bindingSyntax = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var eachExpressionGrammar = ExpressionGrammar{Operators: []string{"in"}, MaxTokens: 3}
var withExpressionGrammar = ExpressionGrammar{Operators: []string{"as"}, MaxTokens: 3}

func (EachDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-each", "data-ht-with"}
	if !strict {
		attributes = append(attributes, "ht-each", "ht-with")
	}
	return attributes
}

func (d EachDirective) eachAttributes(strict bool) []string {
	attributes := []string{"data-ht-each"}
	if !strict {
		attributes = append(attributes, "ht-each")
	}
	return attributes
}

func (d EachDirective) withAttributes(strict bool) []string {
	attributes := []string{"data-ht-with"}
	if !strict {
		attributes = append(attributes, "ht-with")
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

func (EachDirective) Selectors(strict bool) []string {
	if strict {
		return []string{
			"[data-ht-each]:not([data-ht-each] *, [data-ht-with] *)",
			"[data-ht-with]:not([data-ht-each] *, [data-ht-with] *)",
		}
	}
	return []string{
		"[data-ht-each]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
		"[data-ht-with]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
		"[ht-each]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
		"[ht-with]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
	}
}

func (d EachDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (EachDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (EachDirective) ParseExpression(expression string) ([]Token, error) {
	return tokenizeExpression(expression, eachExpressionGrammar)
}

func (d EachDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) (err error) {
	if document == nil || element == nil {
		return nil
	}
	if w == nil {
		w = os.Stdout
	}
	if len(htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...)) == 0 {
		return nil
	}
	var bindings []htdata.TemplateData
	if bindings, err = d.dataBindings(element, data, document.StrictMode); err != nil {
		return err
	}
	for index := range bindings {
		// clone the layout template for mutation
		var implicitApply bool
		layoutFragment := htdocument.CloneElement(element)
		htdocument.RemoveAttributes(layoutFragment, d.allAttributes(false)...) // remove all EachDirective attributes regardless of StrictMode
		if implicitApply = !htdocument.HasAttribute(layoutFragment, ApplyDirective{}.Attributes(document.StrictMode)...); implicitApply {
			htdocument.SetAttribute(layoutFragment, "data-ht-apply", "")
		}

		// process the fragment and insert it into the parent element
		nestedDoc := htdocument.Document{Layout: layoutFragment, Theme: document.Theme, TidyMode: document.TidyMode, MarkdownConfig: document.MarkdownConfig, Plugins: document.Plugins}
		if _, err := Process(&nestedDoc, &bindings[index], userData, w); err != nil {
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

func (d EachDirective) dataBindings(element *html.Node, data *htdata.TemplateData, strict bool) ([]htdata.TemplateData, error) {
	switch d.bindMode(element, strict) {
	case BindNone:
		return nil, nil
	case BindWith:
		return d.withDataBinding(element, data, strict)
	case BindEach:
		return d.eachDataBindings(element, data, strict)
	case BindError:
		fallthrough
	default:
		return nil, fmt.Errorf("ht-each and ht-with cannot be combined on %s", htdocument.ElementString(element))
	}
}

func (d EachDirective) eachDataBindings(element *html.Node, data *htdata.TemplateData, strict bool) (bindings []htdata.TemplateData, err error) {
	var attributes []html.Attribute
	var binding string
	var values []any
	var offset = htdocument.GetAttributeInt(element, d.offsetAttributes(strict)...)
	var limit = htdocument.GetAttributeInt(element, d.limitAttributes(strict)...)
	if attributes = htdocument.GetAttributes(element, d.eachAttributes(strict)...); len(attributes) == 0 {
		return nil, nil
	}
	if binding, values, err = d.expressionParser(attributes[0].Val, data); err == nil {
		bindings = make([]htdata.TemplateData, 0, len(values)) // pre-allocate the binding slice
	} else {
		return nil, d.bindingError(element, attributes[0], err)
	}
	for index, value := range values {
		index++
		count, ok := d.expressionWindow(index, len(values), offset, limit)
		if !ok {
			continue
		}
		nested := data.ShallowClone()
		nested[binding] = value
		nested["each"] = htdata.TemplateData{"index": index, "count": count, "total": len(values)}
		bindings = append(bindings, nested)
	}
	return bindings, nil
}

func (d EachDirective) withDataBinding(element *html.Node, data *htdata.TemplateData, strict bool) ([]htdata.TemplateData, error) {
	var attributes = htdocument.GetAttributes(element, d.withAttributes(strict)...)
	var bindings = []htdata.TemplateData{data.ShallowClone()}
	var err error
	for _, attribute := range attributes {
		var tokens []Token
		var value any
		if tokens, err = tokenizeExpression(attribute.Val, withExpressionGrammar); err != nil {
			return nil, d.bindingError(element, attribute, err)
		}
		if err = d.ValidateWithExpression(tokens); err != nil {
			return nil, d.bindingError(element, attribute, err)
		}
		if value, err = htvariable.ResolveValue(tokens[0].Raw, &bindings[0]); err != nil {
			return nil, d.bindingError(element, attribute, err)
		}
		bindings[0][tokens[2].Raw] = value
	}
	return bindings, nil
}

func (d EachDirective) bindMode(element *html.Node, strict bool) (mode EachDirectiveBindMode) {
	hasEach := len(htdocument.GetAttributes(element, d.eachAttributes(strict)...)) > 0
	hasWith := len(htdocument.GetAttributes(element, d.withAttributes(strict)...)) > 0
	switch {
	case hasEach && hasWith:
		return BindError
	case hasEach:
		return BindEach
	case hasWith:
		return BindWith
	default:
		return BindNone
	}
}

func (EachDirective) bindingError(element *html.Node, attribute html.Attribute, err error) error {
	return fmt.Errorf("invalid %q directive on %s: %w", attribute.Key, htdocument.ElementString(element), err)
}

func (d EachDirective) expressionParser(expression string, data htvariable.Getter) (binding string, values []any, err error) {
	tokens, err := d.ParseExpression(expression)
	if err != nil {
		return "", nil, err
	}
	if err := d.ValidateExpression(tokens); err != nil {
		return "", nil, err
	}
	binding = tokens[0].Raw
	values, err = d.expressionValues(tokens[2].Raw, data)
	if err != nil {
		return "", nil, err
	}
	return binding, values, nil
}

func (d EachDirective) ValidateExpression(tokens Tokens) error {
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

func (EachDirective) ValidateWithExpression(tokens Tokens) error {
	if len(tokens) != 3 {
		return fmt.Errorf("ht-with: invalid syntax (expected \"${ variable } as binding\", got %q)", tokens.Expression())
	}
	if tokens[0].Kind != TokenVariable {
		return fmt.Errorf("ht-with: expected template variable ${ ... } as the first token")
	}
	if tokens[1].Kind != TokenOperator || tokens[1].Raw != "as" {
		return fmt.Errorf("ht-with: expected \"as\" operator, got %q", tokens[1].Raw)
	}
	if tokens[2].Kind != TokenLiteral || tokens[2].Quoted || !bindingSyntax.MatchString(tokens[2].Raw) || tokens[2].Raw == "each" {
		return fmt.Errorf("ht-with: invalid binding %q", tokens[2].Raw)
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
