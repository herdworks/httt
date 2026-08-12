package directives

import (
	"strings"
	"testing"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

type recordingAttributeGetter struct {
	values map[string]any
	calls  []string
}

func (getter *recordingAttributeGetter) Get(key string) any {
	getter.calls = append(getter.calls, key)
	return getter.values[key]
}

type namedAttributeMap map[string]int

func TestAttributeMapDirectiveSkipsReservedMapKeys(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><meta ht-attrs="${ attributes }"></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"attributes": map[string]any{
		"content": "value",
		"ht-if":   "${ page.visible }",
	}}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), `content="value"`) || strings.Contains(string(output), "ht-if") {
		t.Fatalf("reserved map key was not skipped: %s", output)
	}
}

func TestAttributeMapDirectiveAppliesTypedMaps(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		data     htdata.TemplateData
		contains []string
		omits    []string
	}{
		{
			name:     "typed fallback and nil values",
			source:   `<html><body><meta ht-attrs="${ missing, fallback }"></body></html>`,
			data:     htdata.TemplateData{"fallback": map[string]any{"content": "value", "empty": nil}},
			contains: []string{`content="value"`},
			omits:    []string{`empty=`},
		},
		{
			name:     "named string keyed map",
			source:   `<html><body><meta ht-attrs="${ attributes }"></body></html>`,
			data:     htdata.TemplateData{"attributes": namedAttributeMap{"width": 640}},
			contains: []string{`width="640"`},
		},
		{
			name:     "missing and empty maps are no ops",
			source:   `<html><body><meta name="original" ht-attrs="${ missing }" data-ht-attrs="${ empty }"></body></html>`,
			data:     htdata.TemplateData{"empty": map[string]any{}},
			contains: []string{`name="original"`},
			omits:    []string{`ht-attrs`, `data-ht-attrs`},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout, err := html.Parse(strings.NewReader(test.source))
			if err != nil {
				t.Fatal(err)
			}
			output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &test.data, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range test.contains {
				if !strings.Contains(string(output), want) {
					t.Fatalf("output %q does not contain %q", output, want)
				}
			}
			for _, unwanted := range test.omits {
				if strings.Contains(string(output), unwanted) {
					t.Fatalf("output %q contains %q", output, unwanted)
				}
			}
		})
	}
}

func TestAttributeMapDirectiveProcessesMixedAliasesInSourceOrder(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><meta title="original" ht-attrs="${ first }" data-ht-attrs="${ second }"></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{
		"first":  map[string]any{"title": "first", "z": "first"},
		"second": map[string]any{"a": "second", "title": "second"},
	}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := string(output)
	if !strings.Contains(got, `title="second"`) {
		t.Fatalf("later map did not overwrite title: %s", output)
	}
	if strings.Index(got, `z="first"`) > strings.Index(got, `a="second"`) {
		t.Fatalf("new attributes were not appended in source order: %s", output)
	}
}

func TestAttributeMapDirectiveRejectsInvalidSourcesWithoutLookup(t *testing.T) {
	directive := AttributeMapDirective{}
	getter := &recordingAttributeGetter{values: map[string]any{"attributes": map[string]any{"title": "Title"}}}
	for _, source := range []string{
		"attributes",
		"before ${ attributes }",
		`${ text(attributes) }`,
		`${ attributes } after`,
		`${ "literal" }`,
		`${ target: attributes }`,
		`${ attributes | text }`,
	} {
		if _, err := directive.parseExpression(source, getter); err == nil {
			t.Fatalf("parseExpression(%q) succeeded, want error", source)
		}
	}
	if len(getter.calls) != 0 {
		t.Fatalf("invalid sources called Get(%q), want no lookup", getter.calls)
	}
}

func TestAttributeMapDirectiveRejectsNonMapValues(t *testing.T) {
	directive := AttributeMapDirective{}
	for name, value := range map[string]any{
		"string":  "value",
		"slice":   []string{"value"},
		"integer": 1,
	} {
		t.Run(name, func(t *testing.T) {
			getter := &recordingAttributeGetter{values: map[string]any{"value": value}}
			if _, err := directive.parseExpression("${ value }", getter); err == nil {
				t.Fatal("non-map value unexpectedly succeeded")
			}
		})
	}
}
