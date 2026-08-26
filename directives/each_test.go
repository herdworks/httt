package directives

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

func TestEachExpandsTypedWindowAndMetadata(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-each="item in ${ items }" ht-offset="1" ht-limit="1" ht-apply>${ each.index }/${ each.count }/${ each.total }:${ item }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"items": []string{"one", "two", "three"}}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`2/1/3:two`)) || bytes.Contains(output, []byte("ht-each")) {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEachUsesFirstNonStrictAliasInSourceOrder(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-each="item in ${ first }" data-ht-each="item in ${ second }" ht-apply>${ item }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{
		"first":  []string{"first"},
		"second": []string{"second"},
	}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(">first<")) || bytes.Contains(output, []byte(">second<")) {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestEachExpressionWindow(t *testing.T) {
	directive := EachDirective{}
	tests := []struct {
		name       string
		index      int
		valueCount int
		offset     int
		limit      int
		count      int
		ok         bool
	}{
		{name: "unwindowed", index: 1, valueCount: 3, count: 3, ok: true},
		{name: "offset excludes source index", index: 5, valueCount: 10, offset: 5, count: 5},
		{name: "offset includes next source index", index: 6, valueCount: 10, offset: 5, count: 5, ok: true},
		{name: "limit excludes source index", index: 9, valueCount: 10, offset: 5, limit: 3, count: 3},
		{name: "limit includes final source index", index: 8, valueCount: 10, offset: 5, limit: 3, count: 3, ok: true},
		{name: "offset past source", index: 1, valueCount: 3, offset: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count, ok := directive.expressionWindow(test.index, test.valueCount, test.offset, test.limit)
			if count != test.count || ok != test.ok {
				t.Fatalf("expressionWindow() = (%d, %t), want (%d, %t)", count, ok, test.count, test.ok)
			}
		})
	}
}

func TestEachExpressionValuesPreservesIterationShapes(t *testing.T) {
	directive := EachDirective{}
	array := [2]string{"array one", "array two"}
	mapValue := htdata.TemplateData{"key": "value"}
	tests := []struct {
		name  string
		value any
		want  []any
	}{
		{name: "template data set", value: htdata.TemplateDataSet{{"name": "one"}, {"name": "two"}}, want: []any{htdata.TemplateData{"name": "one"}, htdata.TemplateData{"name": "two"}}},
		{name: "template data slice", value: []htdata.TemplateData{{"name": "one"}}, want: []any{htdata.TemplateData{"name": "one"}}},
		{name: "any slice", value: []any{"one", "two"}, want: []any{"one", "two"}},
		{name: "array", value: array, want: []any{"array one", "array two"}},
		{name: "empty slice", value: []string{}, want: []any{}},
		{name: "map scalar", value: mapValue, want: []any{mapValue}},
		{name: "string scalar", value: "one", want: []any{"one"}},
		{name: "number scalar", value: 1, want: []any{1}},
		{name: "nil", value: nil, want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := htdata.TemplateData{"source": test.value}
			values, err := directive.expressionValues("${ source }", &data)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(values, test.want) {
				t.Fatalf("expressionValues() = %#v, want %#v", values, test.want)
			}
		})
	}
}

func TestEachIsolatesBindingsAndIterationContext(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><div ht-each="item in ${ items }"><span ht-each="item in ${ item.children }" ht-apply>${ item }</span></div></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{
		"item":  "outer binding",
		"items": []htdata.TemplateData{{"children": []string{"one", "two"}}},
		"each":  htdata.TemplateData{"existing": "outer metadata"},
	}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`>one<`)) || !bytes.Contains(output, []byte(`>two<`)) {
		t.Fatalf("nested values missing from output: %s", output)
	}
	if got := data.Get("item"); got != "outer binding" {
		t.Fatalf("outer binding = %#v, want preserved value", got)
	}
	if got := data.Get("each.existing"); got != "outer metadata" {
		t.Fatalf("outer each.existing = %#v, want preserved value", got)
	}
	if got := data.Get("each.index"); got != nil {
		t.Fatalf("outer each.index = %#v, want nil", got)
	}
}

func TestEachShadowsOuterPageAndReplacesIterationContext(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><h2 ht-apply>before:${ page.title }</h2><ul ht-each="page in ${ page.feed.pages }"><li ht-apply>${ each.index }:${ page.title }</li></ul><h3 ht-apply>after:${ page.title }</h3></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{
		"page": htdata.TemplateData{
			"title": "outer page",
			"feed":  htdata.TemplateData{"pages": []htdata.TemplateData{{"title": "first page"}, {"title": "second page"}}},
		},
		"each": htdata.TemplateData{"index": "outer index"},
	}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"before:outer page", "1:first page", "2:second page", "after:outer page"} {
		if !bytes.Contains(output, []byte(want)) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if got := data.Get("page.title"); got != "outer page" {
		t.Fatalf("outer page.title = %#v, want outer page", got)
	}
	if got := data.Get("each.index"); got != "outer index" {
		t.Fatalf("outer each.index = %#v, want outer index", got)
	}
}

func TestEachNormalizesLiteralWindowAttributes(t *testing.T) {
	tests := []struct {
		name       string
		attributes string
		want       []string
	}{
		{name: "invalid and negative are unlimited", attributes: `ht-offset="invalid" ht-limit="-1"`, want: []string{"one", "two", "three"}},
		{name: "first mixed offset wins", attributes: `data-ht-offset="1" ht-offset="0"`, want: []string{"two", "three"}},
		{name: "first mixed limit wins", attributes: `ht-limit="1" data-ht-limit="2"`, want: []string{"one"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout, err := html.Parse(strings.NewReader(`<html><body><p ht-each="item in ${ items }" ` + test.attributes + ` ht-apply>${ item }</p></body></html>`))
			if err != nil {
				t.Fatal(err)
			}
			data := htdata.TemplateData{"items": []string{"one", "two", "three"}}
			output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := html.Parse(bytes.NewReader(output))
			if err != nil {
				t.Fatal(err)
			}
			items := htdocument.QueryAll(rendered, "body > p")
			if len(items) != len(test.want) {
				t.Fatalf("rendered %d items, want %d: %s", len(items), len(test.want), output)
			}
			for index, item := range items {
				if got := strings.TrimSpace(item.FirstChild.Data); got != test.want[index] {
					t.Fatalf("item %d = %q, want %q", index, got, test.want[index])
				}
			}
		})
	}
}

func TestEachDefersConditionalBaseAndIncludes(t *testing.T) {
	t.Run("conditional and base", func(t *testing.T) {
		layout, err := html.Parse(strings.NewReader(`<html><body><section ht-each="item in ${ items }" ht-if="${ item.visible }" ht-base="href on ${ item.base }"><a ht-apply href="entry">${ item.name }</a></section></body></html>`))
		if err != nil {
			t.Fatal(err)
		}
		data := htdata.TemplateData{"items": []htdata.TemplateData{
			{"visible": false, "base": "https://example.com/hidden/", "name": "hidden"},
			{"visible": true, "base": "https://example.com/visible/", "name": "visible"},
		}}
		output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(output, []byte("hidden")) || !bytes.Contains(output, []byte(`href="https://example.com/visible/entry"`)) || !bytes.Contains(output, []byte(">visible<")) {
			t.Fatalf("unexpected output: %s", output)
		}
	})

	t.Run("include", func(t *testing.T) {
		document := setupBuiltinTest(t, `<!doctype html><html><body><article ht-each="item in ${ items }"><section ht-include="fragments/card.html"></section></article></body></html>`, map[string]string{
			"fragments/card.html": `<p ht-apply>${ item }</p>`,
		})
		document.TidyMode = true
		data := htdata.TemplateData{"items": []string{"one", "two"}}
		output, err := Process(document, &data, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(output), "<p>one</p>") != 1 || strings.Count(string(output), "<p>two</p>") != 1 {
			t.Fatalf("include did not defer into item clones: %s", output)
		}
	})

	t.Run("include on loop root", func(t *testing.T) {
		document := setupBuiltinTest(t, `<!doctype html><html><body><link-button ht-include="fragments/link-button.html" ht-each="button in ${ buttons }"></link-button></body></html>`, map[string]string{
			"fragments/link-button.html": `<link-button><a ht-apply href="${ button.href }">${ button.title }</a></link-button>`,
		})
		document.TidyMode = true
		data := htdata.TemplateData{"buttons": []htdata.TemplateData{
			{"href": "/one", "title": "One"},
			{"href": "/two", "title": "Two"},
		}}
		output, err := Process(document, &data, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(output), `<a href="/one">One</a>`) != 1 || strings.Count(string(output), `<a href="/two">Two</a>`) != 1 {
			t.Fatalf("root include did not resolve before loop expansion: %s", output)
		}
	})
}

func TestEachImplicitApplyPreservesAttrsSource(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><head><meta ht-each="meta in ${ page.metadata }" ht-attrs="${ meta }"></head></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"page": htdata.TemplateData{"metadata": []htdata.TemplateData{{"name": "og:video", "content": "https://example.com/video.mp4"}}}}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), `<meta content="https://example.com/video.mp4" name="og:video"/>`) {
		t.Fatalf("ht-attrs source was not applied after iteration: %s", output)
	}
}

func TestEachEmptySourcePlaceholderLifecycle(t *testing.T) {
	for _, tidy := range []bool{false, true} {
		t.Run(fmt.Sprintf("tidy=%t", tidy), func(t *testing.T) {
			layout, err := html.Parse(strings.NewReader(`<html><body><p ht-each="item in ${ missing }">placeholder</p></body></html>`))
			if err != nil {
				t.Fatal(err)
			}
			output, err := Process(&htdocument.Document{Layout: layout, TidyMode: tidy}, &htdata.TemplateData{}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if tidy && bytes.Contains(output, []byte("placeholder")) {
				t.Fatalf("tidy placeholder remained: %s", output)
			}
			if !tidy && (!bytes.Contains(output, []byte(`hidden="true"`)) || !bytes.Contains(output, []byte("placeholder"))) {
				t.Fatalf("non-tidy placeholder missing or visible: %s", output)
			}
		})
	}
}

func TestEachRejectsNakedSourceWithoutLookup(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-each="item in page.items"></p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"items": []string{"unexpected"}}
	_, err = Process(&htdocument.Document{Layout: layout}, &data, nil, nil)
	if err == nil {
		t.Fatal("expected typed source error")
	}
}

func TestEachExpressionParser(t *testing.T) {
	directive := EachDirective{}
	data := htdata.TemplateData{
		"page":    htdata.TemplateData{"items": []string{"page item"}},
		"archive": htdata.TemplateData{"items": []string{"archive item"}},
	}
	tests := []struct {
		name       string
		expression string
		binding    string
		values     []any
		wantErr    string
	}{
		{name: "valid", expression: "item in ${ page.items }", binding: "item", values: []any{"page item"}},
		{name: "ht binding", expression: "ht in ${ page.items }", binding: "ht", values: []any{"page item"}},
		{name: "whitespace", expression: "\titem\t in\n${ page.items, archive.items } ", binding: "item", values: []any{"page item"}},
		{name: "uppercase operator", expression: "item IN ${ page.items }", wantErr: "expected iteration operator as the second token"},
		{name: "missing variable", expression: "item in", wantErr: "expected exactly three tokens"},
		{name: "extra literal", expression: "item in ${ page.items } extra", wantErr: "exceeds maximum of 3 tokens"},
		{name: "unterminated variable", expression: "item in ${ page.items", wantErr: "unterminated variable"},
		{name: "wrong token order", expression: "${ page.items } in item", wantErr: "expected iteration binding as the first token"},
		{name: "non-operator second token", expression: "item ${ page.items } in", wantErr: "expected iteration operator as the second token"},
		{name: "non-variable third token", expression: "item in page.items", wantErr: "expected template variable ${ ... } as the third token"},
		{name: "dotted binding", expression: "item.value in ${ page.items }", wantErr: "invalid iteration binding"},
		{name: "reserved each binding", expression: "each in ${ page.items }", wantErr: "invalid iteration binding"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			binding, values, err := directive.expressionParser(test.expression, &data)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("expressionParser() error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if binding != test.binding || !reflect.DeepEqual(values, test.values) {
				t.Fatalf("expressionParser() = (%q, %#v), want (%q, %#v)", binding, values, test.binding, test.values)
			}
		})
	}
}

func TestWithApplyCreatesLocalSourceOrderedBindings(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><section ht-with="${ page.author } as author" ht-with="${ author.name } as name" title="${ name }"><p ht-apply>${ name }</p><span ht-apply>${ name }</span></section><span ht-apply>${ name }</span></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"name": "Outer", "page": htdata.TemplateData{"author": htdata.TemplateData{"name": "Ada"}}}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`<section title="Ada"><p>Ada</p><span>Ada</span></section>`)) {
		t.Fatalf("scoped target did not render: %s", output)
	}
	if !bytes.Contains(output, []byte(`<span>Outer</span>`)) {
		t.Fatalf("binding scope leaked outside the target: %s", output)
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
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
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
	output, err := Process(&htdocument.Document{Layout: layout, StrictMode: true, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`>Strict</p>`)) || bytes.Contains(output, []byte("data-ht-with")) {
		t.Fatalf("strict scoped output: %s", output)
	}
}

func TestWithBindsCollectionsAsOneValue(t *testing.T) {
	for _, value := range []any{[]string{"one", "two"}, htdata.TemplateData{"name": "Ada"}, "text", nil} {
		directive := EachDirective{}
		element := &html.Node{Type: html.ElementNode, Data: "p", Attr: []html.Attribute{{Key: "ht-with", Val: "${ value } as bound"}}}
		data := htdata.TemplateData{"value": value}
		scopes, err := directive.dataBindings(element, &data, false)
		if err != nil || len(scopes) != 1 || !reflect.DeepEqual(scopes[0]["bound"], value) {
			t.Fatalf("scopedData(%#v) = %#v, %v; want one bound scope", value, scopes, err)
		}
	}
}

func TestWithMatchesEachForSingleObjectBinding(t *testing.T) {
	data := htdata.TemplateData{
		"page": htdata.TemplateData{
			"foo": htdata.TemplateData{
				"object": htdata.TemplateData{"title": "Ada", "description": "Single scoped binding"},
			},
		},
	}
	layouts := map[string]string{
		"each": `<article ht-each="binding in ${ page.foo.object }"><h2 ht-apply>${ binding.title }</h2><p ht-apply>${ binding.description }</p></article>`,
		"with": `<article ht-with="${ page.foo.object } as binding"><h2 ht-apply>${ binding.title }</h2><p ht-apply>${ binding.description }</p></article>`,
	}
	outputs := make(map[string][]byte, len(layouts))
	for name, fragment := range layouts {
		layout, err := html.Parse(strings.NewReader(`<html><body>` + fragment + `</body></html>`))
		if err != nil {
			t.Fatal(err)
		}
		output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
		if err != nil {
			t.Fatalf("%s Process() error = %v", name, err)
		}
		outputs[name] = output
	}
	if !bytes.Equal(outputs["with"], outputs["each"]) {
		t.Fatalf("ht-with output = %s, want ht-each output %s", outputs["with"], outputs["each"])
	}
}

func TestWithRejectsEachCombination(t *testing.T) {
	for _, attributes := range []string{
		`ht-each="item in ${ items }" ht-with="${ item } as value"`,
		`data-ht-each="item in ${ items }" data-ht-with="${ item } as value"`,
		`ht-each="item in ${ items }" data-ht-with="${ item } as value"`,
		`data-ht-each="item in ${ items }" ht-with="${ item } as value"`,
	} {
		layout, err := html.Parse(strings.NewReader(`<html><body><p ` + attributes + `>unused</p></body></html>`))
		if err != nil {
			t.Fatal(err)
		}
		data := htdata.TemplateData{"items": []string{"one"}}
		if _, err := Process(&htdocument.Document{Layout: layout}, &data, nil, nil); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatalf("Process(%q) error = %v, want mixed directive error", attributes, err)
		}
	}
}

func TestWithRendersOneTargetForNilBinding(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><section ht-with="${ missing } as bound"><p ht-apply>${ bound, "nil" }</p></section></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &htdata.TemplateData{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(output, []byte(`<section>`)); got != 1 || !bytes.Contains(output, []byte(`<p>nil</p>`)) {
		t.Fatalf("nil scoped binding output = %s, want exactly one rendered target with a nil binding", output)
	}
}

func TestWithAcceptsExplicitApply(t *testing.T) {
	for _, attributes := range []string{
		`ht-with="${ value } as bound" ht-apply`,
		`data-ht-with="${ value } as bound" data-ht-apply`,
	} {
		layout, err := html.Parse(strings.NewReader(`<html><body><p ` + attributes + `>${ bound }</p></body></html>`))
		if err != nil {
			t.Fatal(err)
		}
		data := htdata.TemplateData{"value": "one"}
		output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
		if err != nil {
			t.Fatalf("Process(%q) error = %v", attributes, err)
		}
		if !bytes.Contains(output, []byte(`<p>one</p>`)) {
			t.Fatalf("Process(%q) output = %s, want rendered scoped value", attributes, output)
		}
	}
}

func TestWithScopesConditionalAndBase(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><section ht-with="${ page.card } as card" ht-if="${ card.visible }" ht-base="href on ${ card.base }"><a ht-apply href="entry">${ card.title }</a></section></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"page": htdata.TemplateData{"card": htdata.TemplateData{"visible": true, "base": "/cards", "title": "Ada"}}}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`<a href="/cards/entry">Ada</a>`)) {
		t.Fatalf("scoped conditional/base output: %s", output)
	}
}

func TestWithIncludeForwardsRepeatedBindings(t *testing.T) {
	document := setupBuiltinTest(t,
		`<!doctype html><html><body><example-fragment ht-include="fragments/example.html" ht-with="${ page.foo } as foo" ht-with="${ foo.name } as name"></example-fragment></body></html>`,
		map[string]string{"fragments/example.html": `<example-fragment><p ht-apply>${ name }</p></example-fragment>`},
	)
	document.TidyMode = true
	data := htdata.TemplateData{"page": htdata.TemplateData{"foo": htdata.TemplateData{"name": "Included"}}}
	output, err := Process(document, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`<p>Included</p>`)) || bytes.Contains(output, []byte("ht-with")) {
		t.Fatalf("included binding output: %s", output)
	}
}

func TestWithRejectsInvalidBindingBeforeRendering(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-with="${ missing } as 9bad">${ missing }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{}
	_, err = Process(&htdocument.Document{Layout: layout}, &data, nil, nil)
	if err == nil || !strings.Contains(err.Error(), `invalid "ht-with" directive on <p`) || !strings.Contains(err.Error(), `invalid binding "9bad"`) {
		t.Fatalf("error = %v, want ht-with attribution and invalid binding syntax", err)
	}
}

func TestWithDataBindingResolvesSourceOrderedBindings(t *testing.T) {
	data := htdata.TemplateData{"author": htdata.TemplateData{"name": "Ada"}, "second": "second"}
	directive := EachDirective{}
	if _, err := directive.withDataBinding(nil, &data, false); err != nil {
		t.Fatalf("withDataBinding(nil) error = %v", err)
	}
	element := &html.Node{Type: html.ElementNode, Data: "p", Attr: []html.Attribute{
		{Key: "ht-with", Val: "${ author } as person"},
		{Key: "ht-with", Val: "${ person.name } as name"},
		{Key: "ht-with", Val: "${ second } as name"},
	}}
	bindings, err := directive.withDataBinding(element, &data, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 {
		t.Fatalf("withDataBinding returned %d bindings, want 1", len(bindings))
	}
	if got := bindings[0].Get("name"); got != "second" {
		t.Fatalf("binding = %#v, want final resolved value", got)
	}
	if data.Get("name") != nil {
		t.Fatalf("bindings changed outer data: %#v", data)
	}
	element.Attr = []html.Attribute{{Key: "ht-with", Val: "${ author } as 9bad"}}
	if _, err := directive.withDataBinding(element, &data, false); err == nil {
		t.Fatal("withDataBinding accepted an invalid binding")
	}
}

func TestWithIncludePreservesRepeatedBindingOrder(t *testing.T) {
	placeholder := &html.Node{Type: html.ElementNode, Data: "example", Attr: []html.Attribute{
		{Key: "ht-with", Val: "${ page.author } as author"},
		{Key: "ht-with", Val: "${ author.name } as name"},
	}}
	fragment := &html.Node{Type: html.ElementNode, Data: "section"}
	htdocument.ForwardAttributes(placeholder, []*html.Node{fragment}, (EachDirective{}).withAttributes(false))
	attributes := htdocument.GetAttributes(fragment, "ht-with")
	if len(attributes) != 2 || attributes[0].Val != placeholder.Attr[0].Val || attributes[1].Val != placeholder.Attr[1].Val {
		t.Fatalf("forwarded attributes = %#v, want source-ordered duplicates", attributes)
	}
}

func TestEachLegacyTemplateFailsWithMigrationHint(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-template="item:items"></p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"items": []string{"one"}}
	_, err = Process(&htdocument.Document{Layout: layout}, &data, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "hyperctl theme migrate") {
		t.Fatalf("error = %v, want migration hint", err)
	}
}

func TestDeprecatedParamFailsWithMigrationHint(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-param="page.title"></p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Process(&htdocument.Document{Layout: layout}, &htdata.TemplateData{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "hyperctl theme migrate") {
		t.Fatalf("error = %v, want migration hint", err)
	}
}

func TestDeprecatedQueryFailsWithMigrationHint(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><a ht-query="href?version:site.version"></a></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Process(&htdocument.Document{Layout: layout}, &htdata.TemplateData{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "hyperctl theme migrate") {
		t.Fatalf("error = %v, want migration hint", err)
	}
}

func TestDeprecatedDirectivesAreNotDeferredOrIgnoredInStrictMode(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><div data-ht-each="item in ${ items }"><p ht-content="page.body"></p></div></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"items": []string{}}
	_, err = Process(&htdocument.Document{Layout: layout, StrictMode: true}, &data, nil, nil)
	if err == nil || !strings.Contains(err.Error(), `"ht-content"`) {
		t.Fatalf("error = %v, want stale directive error", err)
	}
}
