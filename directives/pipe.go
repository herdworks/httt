package directives

import (
	"bytes"
	"fmt"
	"hash/maphash"
	"io"
	"slices"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

type PipeDirective struct{}

var _ Directive = PipeDirective{}           // make the compiler check Directive interface conformance
var _ ExpressiveDirective = PipeDirective{} // make the compiler check ExpressiveDirective interface conformance

var pipeExpressionGrammar = ExpressionGrammar{Operators: []string{"from", "as"}, AllowQuotedLiterals: true, MaxTokens: 4}

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
	if document == nil || document.IsFragment() {
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
		return fmt.Errorf("ht-pipe: syntax error (expected 'from \"<selector>\"' or 'from \"<selector>\" as <type>'; got: %q)", tokens.Expression())
	case tokens[0].Kind != TokenOperator || tokens[0].Raw != "from":
		// all ht-pipe expressions must start with the "from" operator
		return fmt.Errorf("ht-pipe: missing \"from\" keyword (expected 'from \"<selector>\"' or 'from \"<selector>\" as <type>'; got: %q)", tokens.Expression())
	case tokens[1].Kind != TokenLiteral || !tokens[1].Quoted || tokens[1].Unquoted() == "":
		// all ht-pipe selectors must be non-empty quoted strings
		return fmt.Errorf("ht-pipe: selector must be a quoted string (expected 'from \"<selector>\"' or 'from \"<selector>\" as <type>'; got: %q)", tokens.Expression())
	case len(tokens) == 2:
		// valid 'from "<selector>"' syntax
		return nil
	case tokens[2].Kind != TokenOperator || tokens[2].Raw != "as" || tokens[3].Kind != TokenLiteral:
		// invalid "as <type>" expression
		return fmt.Errorf("ht-pipe: invalid type expression (got '%s %s', expected 'as <type>')", tokens[2].Raw, tokens[3].Raw)
	case !slices.Contains([]string{"element", "text", "css", "js", "javascript"}, tokens[3].Unquoted()):
		// unsupported pipe type
		return fmt.Errorf("ht-pipe: unsupported pipe type (got %q expected element, text, css, js, or javascript)", tokens[3].Raw)
	case len(tokens) == 4:
		// valid 'from "<selector>" as <type>' syntax
		fallthrough
	default:
		return nil
	}
}

func (d PipeDirective) Process(document *htdocument.Document, element *html.Node, _ *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if document == nil || document.IsFragment() || element == nil {
		return nil
	}
	deduper := pipeDeduper{seed: maphash.MakeSeed(), buckets: map[uint64][][]byte{}}
	attributes := htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...)
	for _, attribute := range attributes {
		var selector, mode string
		if selector, mode, err = d.parseAttribute(attribute.Val); err != nil {
			return fmt.Errorf("error parsing %q directive;\n%s\n%w", attribute.Key, htdocument.ElementString(element), err)
		}
		matches := htdocument.QueryAll(document.Layout, selector)
		unique := make([]*html.Node, 0, len(matches))
		for _, match := range matches {
			if match == element {
				continue
			}
			if match.Parent != nil {
				match.Parent.RemoveChild(match)
			}
			if !deduper.seen(match) {
				unique = append(unique, match)
			}
		}
		for _, match := range unique {
			if mode == "element" {
				element.AppendChild(match)
				continue
			}
			htdocument.AppendText(element, htdocument.TextContent(match, true))
		}
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

// pipeDeduper groups rendered element identities by a fast, process-local hash.
// Exact byte comparisons inside each bucket preserve correctness on collisions.
type pipeDeduper struct {
	seed    maphash.Seed
	buckets map[uint64][][]byte
}

func (d *pipeDeduper) seen(element *html.Node) bool {
	rendered := htdocument.HTML(element)
	var hash maphash.Hash
	hash.SetSeed(d.seed)
	_, _ = hash.Write(rendered)
	key := hash.Sum64()
	for _, existing := range d.buckets[key] {
		if bytes.Equal(existing, rendered) {
			return true
		}
	}
	d.buckets[key] = append(d.buckets[key], rendered)
	return false
}
