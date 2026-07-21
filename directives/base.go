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

func RenderBasedElement(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

	paramString := htdocument.GetAttributeString(element, HyperBaseAttributeStrings...)
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
		if ResolveBasedAttributes(document, element, param.Target, baseURL, w) != nil {
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
	selectors := fmt.Sprintf("[%[1]s]:not([data-ht-template] [%[1]s], [data-ht-template][%[1]s], [data-ht-base] [%[1]s], [data-ht-base][%[1]s])", attribute)
	if !document.StrictMode {
		selectors += fmt.Sprintf(":not([ht-template] [%[1]s], [ht-template][%[1]s], [ht-base] [%[1]s], [ht-base][%[1]s])", attribute)
	}
	for _, el := range htdocument.QueryRoot(element, selectors) {
		var resolvedHref string
		targetHref := htdocument.GetAttributeString(el, attribute)
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

func HyperBaseElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-base]:not([data-ht-template] [data-ht-base], [data-ht-template][data-ht-base])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-base], [ht-template][data-ht-base])"
	selectors = append(selectors, "[ht-base]:not([ht-template] [ht-base], [ht-template][ht-base]):not([data-ht-template] [ht-base], [data-ht-template][ht-base])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
