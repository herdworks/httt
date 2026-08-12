package directives

import (
	"bytes"
	"strings"
	"testing"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

func TestDebugSupportsRepeatedElementAndVariableAttributes(t *testing.T) {
	element := &html.Node{
		Type: html.ElementNode,
		Data: "article",
		Attr: []html.Attribute{
			{Key: "ht-debug", Val: "element"},
			{Key: "ht-debug", Val: "${ page.title }"},
			{Key: "data-ht-debug", Val: "${ page.metadata }"},
		},
	}
	document := &htdocument.Document{Layout: element}
	data := htdata.TemplateData{
		"page": map[string]any{
			"title":    "Example",
			"metadata": map[string]any{"published": true},
		},
	}
	output := &bytes.Buffer{}

	if err := (DebugDirective{}).Process(document, element, &data, nil, output); err != nil {
		t.Fatal(err)
	}

	got := output.String()
	for _, want := range []string{
		"===== ht-debug =====",
		`<article ht-debug="element" ht-debug="${ page.title }" data-ht-debug="${ page.metadata }"></article>`,
		"${ page.title }:\n\"Example\"",
		"${ page.metadata }:\n{\n\t\"published\": true\n}",
		"====================",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("debug output missing %q:\n%s", want, got)
		}
	}
	if htdocument.HasAttribute(element, "ht-debug", "data-ht-debug") {
		t.Fatalf("debug attributes were not removed: %s", document.HTML(element))
	}
}

func TestDebugRejectsNonVariableModes(t *testing.T) {
	for _, expression := range []string{"", "data", "element, data", "page.title"} {
		t.Run(expression, func(t *testing.T) {
			element := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{{Key: "ht-debug", Val: expression}}}
			document := &htdocument.Document{Layout: element}
			data := htdata.TemplateData{"page": map[string]any{"title": "Example"}}

			err := (DebugDirective{}).Process(document, element, &data, nil, &bytes.Buffer{})
			if err == nil || (!strings.Contains(err.Error(), "ht-debug requires exactly one") && !strings.Contains(err.Error(), "maximum of 1 tokens")) {
				t.Fatalf("error = %v, want single-token ht-debug error", err)
			}
			if !htdocument.HasAttribute(element, "ht-debug") {
				t.Fatal("invalid debug attribute was removed")
			}
		})
	}
}
