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

type DebugDirective struct{}

var _ Directive = DebugDirective{} // make the compiler check Directive interface conformance

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

func (d DebugDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, w io.Writer) (err error) {
	if document == nil {
		return nil
	}
	if w == nil {
		w = os.Stdout
	}

	var modes []string
	modesString := strings.ReplaceAll(htdocument.GetAttribute(element, d.Attributes(document.StrictMode)...).Val, " ", "")
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
	htdocument.RemoveAttributes(element, d.Attributes(false)...)
	return nil
}
