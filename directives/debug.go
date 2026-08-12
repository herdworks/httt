package directives

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"github.com/herdworks/httt/variable"
	"golang.org/x/net/html"
)

type DebugDirective struct{}

var _ Directive = DebugDirective{}           // make the compiler check Directive interface conformance
var _ ExpressiveDirective = DebugDirective{} // make the compiler check ExpressiveDirective interface conformance

var debugExpressionGrammar = ExpressionGrammar{Operators: []string{"element"}, MaxTokens: 1}

func (d DebugDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-debug"}
	if !strict {
		attributes = append(attributes, "ht-debug")
	}
	return attributes
}

func (d DebugDirective) Selectors(strict bool) []string {
	return defaultDirectiveSelectors(d, strict)
}

func (d DebugDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (DebugDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (DebugDirective) ParseExpression(expression string) ([]Token, error) {
	return tokenizeExpression(expression, debugExpressionGrammar)
}

func (DebugDirective) ValidateExpression(tokens Tokens) error {
	if len(tokens) != 1 || (tokens[0].Kind != TokenOperator && tokens[0].Kind != TokenVariable) {
		return fmt.Errorf("ht-debug requires exactly one 'element' operator or ${ ... } variable")
	}
	return nil
}

func (d DebugDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, w io.Writer) (err error) {
	if document == nil || element == nil {
		return nil
	}
	if w == nil {
		w = os.Stdout
	}

	statement := fmt.Appendf([]byte{}, "\n===== ht-debug =====\n")
	for _, attribute := range htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...) {
		tokens, err := d.ParseExpression(attribute.Val)
		if err != nil {
			return fmt.Errorf("error parsing %q directive;\n%s\n%w", attribute.Key, htdocument.ElementString(element), err)
		}
		if err := d.ValidateExpression(tokens); err != nil {
			return fmt.Errorf("error parsing %q directive;\n%s\n%w", attribute.Key, htdocument.ElementString(element), err)
		}
		if tokens[0].Kind == TokenOperator {
			statement = fmt.Appendf(statement, "%s\n", document.HTML(element))
			continue
		}
		value, err := variable.ResolveValue(tokens[0].Raw, data)
		if err != nil {
			return fmt.Errorf("error resolving %q directive;\n%s\n%w", attribute.Key, htdocument.ElementString(element), err)
		}
		valueBytes, err := json.MarshalIndent(value, "", "\t")
		if err != nil {
			return fmt.Errorf("error encoding %q directive;\n%s\n%w", attribute.Key, htdocument.ElementString(element), err)
		}
		statement = fmt.Appendf(statement, "%s:\n%s\n", tokens[0].Raw, valueBytes)
	}
	statement = fmt.Appendf(statement, "====================\n")
	fmt.Fprintln(w, string(statement))
	htdocument.RemoveAttributes(element, d.Attributes(false)...)
	return nil
}
