package directives

import (
	"fmt"
	"io"
	"slices"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

type PipeDirective struct{}

var _ Directive = PipeDirective{}           // make the compiler check Directive interface conformance
var _ DirectiveExpression = PipeDirective{} // make the compiler check DirectiveExpression interface conformance

var pipeExpressionGrammar = ExpressionGrammar{Operators: []string{"to", "as"}, AllowQuotedLiterals: true, MaxTokens: 4}

func (d PipeDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-pipe"}
	if !strict {
		attributes = append(attributes, "ht-pipe")
	}
	return attributes
}

func (d PipeDirective) Selectors(strict bool) []string {
	selectors := []string{"[data-ht-pipe]"}
	if !strict {
		selectors = append(selectors, "[ht-pipe]")
	}
	return selectors
}

func (d PipeDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (d PipeDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (d PipeDirective) ParseExpression(expression string) ([]Token, error) {
	return tokenizeExpression(expression, pipeExpressionGrammar)
}

func (d PipeDirective) ValidateExpression(tokens Tokens) error {
	switch {
	case len(tokens) != 2 && len(tokens) != 4:
		// syntax error
		return fmt.Errorf("ht-pipe: syntax error (expected 'to \"<selector>\"' or 'to \"<selector>\" as <type>'; got: %q)", tokens.Expression())
	case tokens[0].Kind != TokenOperator || tokens[0].Raw != "to":
		// all ht-pipe expressions must start with the "to" operator
		return fmt.Errorf("ht-pipe: missing \"to\" keyword (expected 'to \"<selector>\"' or 'to \"<selector>\" as <type>'; got: %q)", tokens.Expression())
	case tokens[1].Kind != TokenLiteral || !tokens[1].Quoted || tokens[1].Unquoted() == "":
		// all ht-pipe selectors must be non-empty quoted strings
		return fmt.Errorf("ht-pipe: selector must be a quoted string (expected 'to \"<selector>\"' or 'to \"<selector>\" as <type>'; got: %q)", tokens.Expression())
	case len(tokens) == 2:
		// valid 'to "<selector>"' syntax
		return nil
	case tokens[2].Kind != TokenOperator || tokens[2].Raw != "as" || tokens[3].Kind != TokenLiteral:
		// invalid "as <type>" expression
		return fmt.Errorf("ht-pipe: invalid type expression (got '%s %s', expected 'as <type>')", tokens[2].Raw, tokens[3].Raw)
	case !slices.Contains([]string{"element", "text", "css", "js", "javascript"}, tokens[3].Unquoted()):
		// unsupported pipe type
		return fmt.Errorf("ht-pipe: unsupported pipe type (got %q expected element, text, css, js, or javascript)", tokens[3].Raw)
	case len(tokens) == 4:
		// valid 'to "<selector>" as <type>' syntax
		fallthrough
	default:
		return nil
	}
}

func (d PipeDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if document == nil || element == nil || element.Parent == nil {
		return nil
	}
	attributes := htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...)
	for _, attribute := range attributes {
		var selector, mode string
		var destination *html.Node
		if selector, mode, err = d.parseAttribute(attribute.Val); err != nil {
			return fmt.Errorf("error parsing %q directive;\n%s\n%w", attribute.Key, htdocument.ElementString(element), err)
		}
		if destination = htdocument.Query(document.Layout, selector); destination == nil {
			continue // try the next ht-pipe attribute, if any
		} else if mode != "element" {
			element.Parent.RemoveChild(element)
			htdocument.AppendText(destination, htdocument.TextContent(element, true))
			break // stop processing ht-pipe directives after first successful pipe
		} else {
			element.Parent.RemoveChild(element)
			destination.AppendChild(element)
			break // stop processing ht-pipe directives after first successful pipe
		}
	}
	if document.BlockMode {
		return nil
	}
	if document.TidyMode {
		htdocument.RemoveAttributes(element, d.Attributes(false)...)
	}
	return nil
}

func (d PipeDirective) parseAttribute(source string) (selector string, mode string, err error) {
	var tokens Tokens
	if tokens, err = d.ParseExpression(source); err != nil {
		return "", "", err
	}
	if err := d.ValidateExpression(tokens); err != nil {
		return "", "", err
	}
	if len(tokens) == 4 {
		mode = tokens[3].Unquoted()
	} else {
		mode = "element"
	}
	return tokens[1].Unquoted(), mode, nil
}
