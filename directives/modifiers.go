package directives

import (
	"fmt"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	htvariable "github.com/herdworks/httt/variable"
	"golang.org/x/net/html"
)

// WithModifier identifies ht-with attributes when they modify ht-apply or
// ht-each. A remaining modifier is processed as an implicit ht-apply during
// ApplyDirective's follow-up pass.
type WithModifier struct{}

var _ AttributeProvider = WithModifier{} // make the compiler check AttributeProvider interface conformance

var withExpressionGrammar = ExpressionGrammar{Operators: []string{"as"}, MaxTokens: 3}

func (d WithModifier) Attributes(strict bool) []string {
	attributes := []string{"data-ht-with"}
	if !strict {
		attributes = append(attributes, "ht-with")
	}
	return attributes
}

func (d WithModifier) Selectors(strict bool) []string {
	return defaultDirectiveSelectors(d, strict)
}

func (d WithModifier) ValidateExpression(tokens Tokens) error {
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

// Bindings resolves an element's active ht-with attributes directly into data.
// Callers should pass an already-shallow local copy. Attributes resolve in
// source order so later declarations can refer to an earlier binding; duplicate
// names use the final declaration.
func (d WithModifier) Bindings(element *html.Node, data *htdata.TemplateData, strict bool) (err error) {
	attributes := htdocument.GetAttributes(element, d.Attributes(strict)...)
	if len(attributes) == 0 || data == nil {
		return nil
	}
	for _, attribute := range attributes {
		var tokens Tokens
		var value any
		if tokens, err = tokenizeExpression(attribute.Val, withExpressionGrammar); err != nil {
			return err
		}
		if err = d.ValidateExpression(tokens); err != nil {
			return fmt.Errorf("invalid %q modifier: %w", attribute.Key, err)
		}
		if value, err = htvariable.ResolveValue(tokens[0].Raw, data); err != nil {
			return err
		}
		(*data)[tokens[2].Raw] = value // bind value
	}
	return nil
}
