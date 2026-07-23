package directives

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperQueryAttributeStrings = []string{"data-ht-query", "ht-query"}

type QueryDirective struct{}

var _ Directive = QueryDirective{} // make the compiler check Directive interface conformance

func (d QueryDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-query"}
	if !strict {
		attributes = append(attributes, "ht-query")
	}
	return attributes
}

func (d QueryDirective) Selectors(strict bool) []string {
	selectors := []string{"[data-ht-query]:not([data-ht-template] [data-ht-query], [data-ht-template][data-ht-query])"}
	if strict {
		return selectors
	}
	selectors[0] += ":not([ht-template] [data-ht-query], [ht-template][data-ht-query])"
	return append(selectors, "[ht-query]:not([ht-template] [ht-query], [ht-template][ht-query]):not([data-ht-template] [ht-query], [data-ht-template][ht-attrs])")
}

func (d QueryDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (d QueryDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, w io.Writer) (err error) {
	if document == nil {
		return nil
	}
	if w == nil {
		w = os.Stdout
	}

	debug := htdocument.HasAttribute(element, HyperDebugAttributeStrings...)
	paramString := htdocument.GetAttribute(element, HyperQueryAttributeStrings...).Val
	for _, param := range ParseParameters(paramString, data) {
		var attributeName string
		var attributeQuery string
		var hasQuery bool
		if param.Value() == nil {
			continue
		}
		if attributeName, attributeQuery, hasQuery = strings.Cut(param.Target, "?"); !hasQuery {
			continue
		}
		if err = setAttributeQueryParam(element, attributeName, attributeQuery, fmt.Sprintf("%v", param.Value()), debug, w); err != nil {
			return err
		}
		htdocument.RemoveAttributes(element, HyperQueryAttributeStrings...)
	}
	return nil
}

func setAttributeQueryParam(element *html.Node, attribute string, queryParam string, value string, debug bool, w io.Writer) (err error) {
	var attributeURL *url.URL
	attributeHref := htdocument.GetAttribute(element, attribute).Val
	if attributeURL, err = url.Parse(attributeHref); err != nil {
		return err
	}
	query := attributeURL.Query()
	query.Set(queryParam, value)
	attributeURL.RawQuery = query.Encode()
	if debug {
		fmt.Fprintf(w, "urlquery %s => %s updating\n", attributeHref, attributeURL.String())
	}
	htdocument.SetAttribute(element, attribute, attributeURL.String())
	return nil
}
