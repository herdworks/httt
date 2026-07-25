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

var HyperBaseAttributeStrings = []string{"data-ht-base", "ht-base"}

type BaseDirective struct{}

var _ Directive = BaseDirective{} // make the compiler check Directive interface conformance

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

func (d BaseDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, w io.Writer) (err error) {
	if document == nil {
		return nil
	}
	if w == nil {
		w = os.Stdout
	}

	paramString := htdocument.GetAttribute(element, HyperBaseAttributeStrings...).Val
	for _, param := range ParseParameters(paramString, data) {
		if len(param.Values) == 0 {
			continue
		}
		htdocument.RemoveAttributes(element, HyperBaseAttributeStrings...)
		var baseURL *url.URL
		var baseHref string = fmt.Sprintf("%s", param.Value())
		if !(strings.HasSuffix(baseHref, "/") || strings.HasSuffix(baseHref, "index.html")) {
			baseHref += "/"
		}
		if baseURL, err = url.Parse(baseHref); err != nil {
			return nil
		}
		if err = ResolveBasedAttributes(document, element, param.Target, baseURL, w); err != nil {
			return err
		}
	}
	return nil
}

func ResolveBasedAttributes(document *htdocument.Document, element *html.Node, attribute string, baseURL *url.URL, w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

	debug := htdocument.HasAttribute(element, HyperDebugAttributeStrings...)
	selectors := fmt.Sprintf("[%[1]s]:not([data-ht-each] [%[1]s], [data-ht-each][%[1]s], [data-ht-base] [%[1]s], [data-ht-base][%[1]s])", attribute)
	if !document.StrictMode {
		selectors += fmt.Sprintf(":not([ht-each] [%[1]s], [ht-each][%[1]s], [ht-base] [%[1]s], [ht-base][%[1]s])", attribute)
	}
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
