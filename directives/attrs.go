package directives

import (
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"github.com/herdworks/httt/variable"
	"golang.org/x/net/html"
)

type AttributeMapDirective struct{}

type Attr struct {
	key   string
	value any
}

var _ Directive = AttributeMapDirective{}           // make the compiler check Directive interface conformance
var _ ExpressiveDirective = AttributeMapDirective{} // make the compiler check ExpressiveDirective interface conformance

var attributeMapExpressionGrammar = ExpressionGrammar{MaxTokens: 1}

func (d AttributeMapDirective) Attributes(strict bool) []string {
	if strict {
		return []string{"data-ht-attrs"}
	}
	return []string{"data-ht-attrs", "ht-attrs"}
}

func (d AttributeMapDirective) Selectors(strict bool) []string {
	return defaultDirectiveSelectors(d, strict)
}

func (d AttributeMapDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (AttributeMapDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (AttributeMapDirective) ParseExpression(expression string) ([]Token, error) {
	return tokenizeExpression(expression, attributeMapExpressionGrammar)
}

func (AttributeMapDirective) ValidateExpression(tokens Tokens) error {
	if len(tokens) != 1 || tokens[0].Kind != TokenVariable {
		return fmt.Errorf("expected exactly one ${ ... } variable")
	}
	return nil
}

func (d AttributeMapDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) error {
	directives := htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...)
	attributes, err := d.parseDirectives(directives, data)
	if err != nil {
		return fmt.Errorf("invalid attribute map on %s: %w", htdocument.ElementString(element), err)
	}
	for _, attr := range attributes {
		if d.skipAttribute(attr.key) || attr.value == nil {
			continue
		}
		htdocument.SetAttribute(element, attr.key, fmt.Sprint(attr.value))
	}
	if document.TidyMode {
		htdocument.RemoveAttributes(element, d.Attributes(false)...)
	}
	return nil
}

func (d AttributeMapDirective) parseDirectives(directives []html.Attribute, data variable.Getter) (values []Attr, err error) {
	for _, directive := range directives {
		parsed, err := d.parseExpression(directive.Val, data)
		if err != nil {
			return nil, fmt.Errorf("%q: %w", directive.Key, err)
		}
		values = append(values, parsed...)
	}
	return values, nil
}

func (d AttributeMapDirective) parseExpression(expression string, data variable.Getter) ([]Attr, error) {
	tokens, err := d.ParseExpression(expression)
	if err != nil {
		return nil, err
	}
	if err := d.ValidateExpression(tokens); err != nil {
		return nil, err
	}
	value, err := variable.ResolveValue(tokens[0].Raw, data)
	if err != nil || value == nil {
		return nil, err
	}
	mapValue := reflect.ValueOf(value)
	if mapValue.Kind() != reflect.Map || mapValue.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("expected a string-keyed map")
	}
	if mapValue.IsNil() {
		return nil, nil
	}
	values := make([]Attr, 0, mapValue.Len())
	iter := mapValue.MapRange()
	for iter.Next() {
		values = append(values, Attr{
			key:   iter.Key().String(),
			value: iter.Value().Interface(),
		})
	}
	sort.Slice(values, func(left, right int) bool { return values[left].key < values[right].key })
	return values, nil
}

func (d AttributeMapDirective) skipAttribute(key string) bool {
	for _, reserved := range d.reservedAttributeNames() {
		if strings.EqualFold(key, reserved) {
			return true
		}
	}
	return false
}

func (d AttributeMapDirective) reservedAttributeNames() []string {
	return []string{
		"data-ht-not", "ht-not",
		"data-ht-template", "ht-template",
		"data-ht-content", "ht-content",
		"data-ht-param", "ht-param",
		"data-ht-query", "ht-query",
		"data-ht-attr", "ht-attr",
		"data-ht-attrs", "ht-attrs",
		"data-ht-if", "ht-if",
		"data-ht-debug", "ht-debug",
		"data-ht-pipe", "ht-pipe",
		"data-ht-each", "ht-each",
		"data-ht-offset", "ht-offset",
		"data-ht-limit", "ht-limit",
		"data-ht-apply", "ht-apply",
		"data-ht-with", "ht-with",
		"data-ht-block", "ht-block",
		"data-ht-base", "ht-base",
		"data-ht-include", "ht-include",
	}
}
