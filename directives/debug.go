package directives

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperDebugAttributeStrings = []string{"data-ht-debug", "ht-debug"}

func Debug(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, w io.Writer) (err error) {
	if w == nil {
		w = os.Stdout
	}

	var modes []string
	modesString := strings.ReplaceAll(htdocument.GetAttributeString(element, HyperDebugAttributeStrings...), " ", "")
	if len(modesString) > 0 {
		modes = strings.Split(modesString, ",")
	}
	if len(modes) == 0 {
		return nil
	}
	statement := fmt.Appendf([]byte{}, "\n===== ht-debug =====\n")
	if slices.Contains(modes, "element") {
		statement = fmt.Appendf(statement, "%s\n", document.HTML(element))
	}
	if slices.Contains(modes, "data") {
		dataBytes, _ := json.MarshalIndent(&data, "", "\t")
		statement = fmt.Appendf(statement, "%s\n", dataBytes)
	}
	statement = fmt.Appendf(statement, "====================\n")
	fmt.Fprintln(w, string(statement))
	htdocument.RemoveAttributes(element, HyperDebugAttributeStrings...)
	return nil
}

func HyperDebugElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-debug]:not([data-ht-template] [data-ht-debug], [data-ht-template][data-ht-debug])"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-debug], [ht-template][data-ht-debug])"
	selectors = append(selectors, "[ht-debug]:not([ht-template] [ht-debug], [ht-template][ht-debug]):not([data-ht-template] [ht-debug], [data-ht-template][ht-debug])")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
