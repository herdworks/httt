package directives

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"github.com/herdworks/httt/variable"
	"golang.org/x/net/html"
)

type BaseDirective struct{}

var _ Directive = BaseDirective{}           // make the compiler check Directive interface conformance
var _ ExpressiveDirective = BaseDirective{} // make the compiler check ExpressiveDirective interface conformance

var baseExpressionGrammar = ExpressionGrammar{Operators: []string{"on"}, MaxTokens: 3}

func (d BaseDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-base"}
	if !strict {
		attributes = append(attributes, "ht-base")
	}
	return attributes
}

func (d BaseDirective) Selectors(strict bool) []string {
	return defaultDirectiveSelectors(d, strict)
}

func (d BaseDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (BaseDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (BaseDirective) ParseExpression(expression string) ([]Token, error) {
	return tokenizeExpression(expression, baseExpressionGrammar)
}

func (BaseDirective) ValidateExpression(tokens Tokens) error {
	if len(tokens) != 3 || tokens[0].Kind != TokenLiteral || tokens[0].Quoted || tokens[1].Kind != TokenOperator || tokens[1].Raw != "on" || tokens[2].Kind != TokenVariable {
		return fmt.Errorf("ht-base requires '<attribute> on ${ ... }'")
	}
	if strings.Contains(tokens[0].Raw, ";") || strings.TrimSpace(tokens[0].Raw) == "" {
		return fmt.Errorf("ht-base target attribute is invalid")
	}
	return nil
}

func (d BaseDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, w io.Writer) (err error) {
	if document == nil {
		return nil
	}
	if w == nil {
		w = os.Stdout
	}

	type expression struct{ target, base string }
	attributes := htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...)
	expressions := make([]expression, 0, len(attributes))
	targets := map[string]bool{}
	for _, attribute := range attributes {
		target, source, parseErr := d.parseExpression(attribute.Val)
		if parseErr != nil {
			return fmt.Errorf("invalid %q directive (expected %q, got %q)\n%s", attribute.Key, "<attribute> on ${ ... }", attribute.Val, htdocument.ElementString(element))
		}
		if targets[strings.ToLower(target)] {
			return fmt.Errorf("invalid %q directive on %s: duplicate target attribute %q", attribute.Key, htdocument.ElementString(element), target)
		}
		targets[strings.ToLower(target)] = true
		base, renderErr := variable.RenderString(source, data, variable.SyntaxDefault, document.Plugins.FuncMap())
		if renderErr != nil {
			return wrapVariableError(renderErr, fmt.Sprintf("%q directive", attribute.Key), element)
		}
		expressions = append(expressions, expression{target, base})
	}
	htdocument.RemoveAttributes(element, d.Attributes(false)...)
	for _, expression := range expressions {
		var baseURL *url.URL
		var baseHref string = expression.base
		if !(strings.HasSuffix(baseHref, "/") || strings.HasSuffix(baseHref, "index.html")) {
			baseHref += "/"
		}
		if baseURL, err = url.Parse(baseHref); err != nil {
			return nil
		}
		if err = ResolveBasedAttributes(document, element, expression.target, baseURL, w); err != nil {
			return err
		}
	}
	return nil
}

func (d BaseDirective) parseExpression(source string) (string, string, error) {
	tokens, err := d.ParseExpression(source)
	if err != nil {
		return "", "", err
	}
	if err := d.ValidateExpression(tokens); err != nil {
		return "", "", err
	}
	return tokens[0].Raw, tokens[2].Raw, nil
}

func ResolveBasedAttributes(document *htdocument.Document, element *html.Node, attribute string, baseURL *url.URL, w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

	debug := htdocument.HasAttribute(element, DebugDirective{}.Attributes(document.StrictMode)...)
	selectors := basedAttributeSelector(attribute, document.StrictMode)
	for _, el := range htdocument.QueryRoot(element, selectors) {
		var resolvedHref string
		targetHref := htdocument.GetAttribute(el, attribute).Val
		if debug {
			fmt.Fprintf(w, "base %s='%s' resolving\n", attribute, targetHref)
		}
		if strings.HasPrefix(targetHref, "data:") {
			continue
		}
		if htdocument.IsAbsoluteHref(targetHref) {
			continue
		}
		targetURL := baseURL.ResolveReference(&url.URL{Path: targetHref})
		if resolvedHref, err = url.PathUnescape(targetURL.String()); err != nil {
			return err
		}
		if debug {
			fmt.Fprintf(w, "base %[1]s='%[2]s' => %[1]s='%[3]s' resolved\n", attribute, targetHref, resolvedHref)
		}
		htdocument.SetAttribute(el, attribute, resolvedHref)
	}
	return nil
}

func basedAttributeSelector(attribute string, strict bool) string {
	boundaries := baseBoundaryAttributes(strict)
	return fmt.Sprintf(
		"[%s]%s",
		attribute,
		selectorBoundaries(boundaries, true),
	)
}
