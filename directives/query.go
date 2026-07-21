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

func RenderQueryParams(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

	debug := htdocument.HasAttribute(element, HyperDebugAttributeStrings...)
	paramString := htdocument.GetAttributeString(element, HyperQueryAttributeStrings...)
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

func HyperQueryElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-query]:not([data-ht-template] [data-ht-query], [data-ht-template][data-ht-query])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-query], [ht-template][data-ht-query])"
	selectors = append(selectors, "[ht-query]:not([ht-template] [ht-query], [ht-template][ht-query]):not([data-ht-template] [ht-query], [data-ht-template][ht-attrs])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}

func setAttributeQueryParam(element *html.Node, attribute string, queryParam string, value string, debug bool, w io.Writer) (err error) {
	var attributeURL *url.URL
	attributeHref := htdocument.GetAttributeString(element, attribute)
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
