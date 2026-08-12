package directives

import (
	"bytes"
	"maps"
	"strings"
	"testing"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

func TestWithApplyCreatesLocalSourceOrderedBindings(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-apply ht-with="${ page.author } as author" ht-with="${ author.name } as name" title="${ name }">${ name }</p><span ht-apply>${ name }</span></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"name": "Outer", "page": htdata.TemplateData{"author": htdata.TemplateData{"name": "Ada"}}}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`<p title="Ada">Ada</p>`)) {
		t.Fatalf("scoped target did not render: %s", output)
	}
	if !bytes.Contains(output, []byte(`>Outer</span>`)) {
		t.Fatalf("binding scope leaked to descendant: %s", output)
	}
	if data.Get("name") != "Outer" {
		t.Fatalf("binding changed outer data: %#v", data)
	}
}

func TestWithWithoutApplyUsesImplicitApply(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-with="${ page.title } as title" title="${ title }">${ title }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"page": htdata.TemplateData{"title": "Implicit"}}
	output, err := Process(&htdocument.Document{Layout: layout}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`<p title="Implicit">Implicit</p>`)) || bytes.Contains(output, []byte("ht-with")) {
		t.Fatalf("implicit apply output: %s", output)
	}
}

func TestWithStrictModeUsesDataAlias(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p data-ht-with="${ page.title } as title">${ title }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"page": htdata.TemplateData{"title": "Strict"}}
	output, err := Process(&htdocument.Document{Layout: layout, StrictMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`>Strict</p>`)) || bytes.Contains(output, []byte("ht-with")) {
		t.Fatalf("strict modifier output: %s", output)
	}
}

func TestWithEachSeesIterationBindingAndMetadata(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-each="item in ${ items }" ht-with="${ item.name } as name" ht-with="${ each.index } as position" ht-apply>${ position }:${ name }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"items": []htdata.TemplateData{{"name": "first"}, {"name": "second"}}}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"1:first", "2:second"} {
		if !bytes.Contains(output, []byte(want)) {
			t.Fatalf("output missing %q: %s", want, output)
		}
	}
	if data.Get("name") != nil || data.Get("position") != nil {
		t.Fatalf("each bindings leaked to outer data: %#v", data)
	}
}

func TestWithEachBindingsRemainLocalToEachIteration(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><ul><li ht-each="item in ${ items }" ht-with="${ item.name } as name" ht-apply>${ name }</li></ul><p ht-apply>${ name }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{
		"name": "Outer",
		"items": []any{
			htdata.TemplateData{"name": "First"},
			htdata.TemplateData{"name": "Second"},
		},
	}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`<li>First</li><li>Second</li>`)) {
		t.Fatalf("iteration bindings did not stay distinct: %s", output)
	}
	if !bytes.Contains(output, []byte(`<p>Outer</p>`)) {
		t.Fatalf("iteration binding leaked outside ht-each: %s", output)
	}
	if data.Get("name") != "Outer" {
		t.Fatalf("iteration binding changed outer data: %#v", data)
	}
}

func TestWithEachEmptySourceDoesNotEvaluateModifier(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-each="item in ${ items }" ht-with="${ item.name } as 9bad">unused</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"items": []htdata.TemplateData{}}
	if _, err := Process(&htdocument.Document{Layout: layout}, &data, nil, nil); err != nil {
		t.Fatalf("empty loop evaluated modifier: %v", err)
	}
}

func TestApplyWithNilDataUsesFallback(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-apply>${ page.title, "Fallback" }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	output, err := Process(&htdocument.Document{Layout: layout}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`>Fallback</p>`)) {
		t.Fatalf("nil-data fallback output: %s", output)
	}
}

func TestWithIncludeForwardsRepeatedBindings(t *testing.T) {
	document := setupBuiltinTest(t,
		`<!doctype html><html><body><example-fragment ht-include="fragments/example.html" ht-apply ht-with="${ page.foo } as foo" ht-with="${ foo.name } as name"></example-fragment></body></html>`,
		map[string]string{"fragments/example.html": `<example-fragment>${ name }</example-fragment>`},
	)
	data := htdata.TemplateData{"page": htdata.TemplateData{"foo": htdata.TemplateData{"name": "Included"}}}
	output, err := Process(document, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`>Included</example-fragment>`)) || bytes.Contains(output, []byte("ht-with")) {
		t.Fatalf("included binding output: %s", output)
	}
}

func TestWithRejectsInvalidBindingBeforeRendering(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-apply ht-with="${ missing } as 9bad">${ missing }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{}
	_, err = Process(&htdocument.Document{Layout: layout}, &data, nil, nil)
	if err == nil || !strings.Contains(err.Error(), `invalid binding "9bad"`) {
		t.Fatalf("error = %v, want invalid binding syntax", err)
	}
}

func TestWithModifierBindingsIsNoOpForNoAttributesAndResolvesInSourceOrder(t *testing.T) {
	data := htdata.TemplateData{"author": htdata.TemplateData{"name": "Ada"}, "second": "second"}
	modifier := WithModifier{}
	if err := modifier.Bindings(nil, &data, false); err != nil {
		t.Fatalf("Bindings(nil) error = %v", err)
	}
	shallow := htdata.TemplateData(maps.Clone(data))
	element := &html.Node{Type: html.ElementNode, Data: "p", Attr: []html.Attribute{
		{Key: "ht-with", Val: "${ author } as person"},
		{Key: "ht-with", Val: "${ person.name } as name"},
		{Key: "ht-with", Val: "${ second } as name"},
	}}
	if err := modifier.Bindings(element, &shallow, false); err != nil {
		t.Fatal(err)
	}
	if got := shallow.Get("name"); got != "second" {
		t.Fatalf("binding = %#v, want final resolved value", got)
	}
	if data.Get("name") != nil {
		t.Fatalf("bindings changed outer data: %#v", data)
	}
	element.Attr = []html.Attribute{{Key: "ht-with", Val: "${ author } as 9bad"}}
	if err := modifier.Bindings(element, &shallow, false); err == nil {
		t.Fatal("Bindings accepted an invalid binding")
	}
}

func TestForwardAttributesPreservesRepeatedWithOrder(t *testing.T) {
	placeholder := &html.Node{Type: html.ElementNode, Data: "example", Attr: []html.Attribute{
		{Key: "ht-with", Val: "${ page.author } as author"},
		{Key: "ht-with", Val: "${ author.name } as name"},
	}}
	fragment := &html.Node{Type: html.ElementNode, Data: "section"}
	htdocument.ForwardAttributes(placeholder, []*html.Node{fragment}, WithModifier{}.Attributes(false))
	attributes := htdocument.GetAttributes(fragment, "ht-with")
	if len(attributes) != 2 || attributes[0].Val != placeholder.Attr[0].Val || attributes[1].Val != placeholder.Attr[1].Val {
		t.Fatalf("forwarded attributes = %#v, want source-ordered duplicates", attributes)
	}
}
