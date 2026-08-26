package directives

import (
	"io"
	"slices"
	"strings"
	"testing"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

func TestAttributeNamesIncludesLegacyAttributesWhenRequested(t *testing.T) {
	withoutLegacy := AttributeNames(false, false)
	if !slices.Contains(withoutLegacy, "data-ht-apply") {
		t.Fatalf("AttributeNames(false, false) = %#v, missing current attribute", withoutLegacy)
	}
	if slices.Contains(withoutLegacy, "data-ht-content") {
		t.Fatalf("AttributeNames(false, false) = %#v, includes legacy attribute", withoutLegacy)
	}

	withLegacy := AttributeNames(false, true)
	if !slices.Contains(withLegacy, "data-ht-content") {
		t.Fatalf("AttributeNames(false, true) = %#v, missing legacy attribute", withLegacy)
	}
}

func TestDirectiveSelectorsMatchLegacyQueries(t *testing.T) {
	tests := []struct {
		name      string
		directive interface{ Selectors(bool) []string }
		strict    []string
		legacy    []string
	}{
		{
			name:      "deprecated",
			directive: DeprecatedDirective{},
			strict: []string{
				"[data-ht-not]", "[ht-not]",
				"[data-ht-template]", "[ht-template]",
				"[data-ht-attr]", "[ht-attr]",
				"[data-ht-content]", "[ht-content]",
				"[data-ht-param]", "[ht-param]",
				"[data-ht-query]", "[ht-query]",
			},
			legacy: []string{
				"[data-ht-not]", "[ht-not]",
				"[data-ht-template]", "[ht-template]",
				"[data-ht-attr]", "[ht-attr]",
				"[data-ht-content]", "[ht-content]",
				"[data-ht-param]", "[ht-param]",
				"[data-ht-query]", "[ht-query]",
			},
		},
		{
			name:      "include",
			directive: IncludeDirective{},
			strict: []string{
				"[data-ht-include]:not([data-ht-each] *, [data-ht-with] *)",
			},
			legacy: []string{
				"[data-ht-include]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
				"[ht-include]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
			},
		},
		{
			name:      "each",
			directive: EachDirective{},
			strict: []string{
				"[data-ht-each]:not([data-ht-each] *, [data-ht-with] *)",
				"[data-ht-with]:not([data-ht-each] *, [data-ht-with] *)",
			},
			legacy: []string{
				"[data-ht-each]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
				"[data-ht-with]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
				"[ht-each]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
				"[ht-with]:not([data-ht-each] *, [data-ht-with] *, [ht-each] *, [ht-with] *)",
			},
		},
		{
			name:      "block",
			directive: BlockDirective{},
			strict:    []string{"[data-ht-block]"},
			legacy:    []string{"[data-ht-block]", "[ht-block]"},
		},
		{
			name:      "pipe",
			directive: PipeDirective{},
			strict:    []string{"[data-ht-pipe]"},
			legacy:    []string{"[data-ht-pipe]", "[ht-pipe]"},
		},
		{
			name:      "base",
			directive: BaseDirective{},
			strict: []string{
				"[data-ht-base]:not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *)",
			},
			legacy: []string{
				"[data-ht-base]:not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *, [ht-each], [ht-each] *, [ht-with], [ht-with] *)",
				"[ht-base]:not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *, [ht-each], [ht-each] *, [ht-with], [ht-with] *)",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.directive.Selectors(true); !slices.Equal(got, test.strict) {
				t.Fatalf("strict selectors = %v, want %v", got, test.strict)
			}
			if got := test.directive.Selectors(false); !slices.Equal(got, test.legacy) {
				t.Fatalf("legacy selectors = %v, want %v", got, test.legacy)
			}
		})
	}
}

func TestBoundarySelectorHelpers(t *testing.T) {
	if got, want := defaultBoundaryAttributes(true), []string{"data-ht-each", "data-ht-with"}; !slices.Equal(got, want) {
		t.Fatalf("strict default boundaries = %v, want %v", got, want)
	}
	if got, want := defaultBoundaryAttributes(false), []string{"data-ht-each", "data-ht-with", "ht-each", "ht-with"}; !slices.Equal(got, want) {
		t.Fatalf("legacy default boundaries = %v, want %v", got, want)
	}
	if got, want := baseBoundaryAttributes(false), []string{"data-ht-each", "data-ht-with", "data-ht-base", "ht-each", "ht-with", "ht-base"}; !slices.Equal(got, want) {
		t.Fatalf("legacy base boundaries = %v, want %v", got, want)
	}

	boundaries := defaultBoundaryAttributes(true)
	if got, want := selectorBoundaries(boundaries, true), ":not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *)"; got != want {
		t.Fatalf("boundary roots and descendants selector = %q, want %q", got, want)
	}
	if got, want := selectorBoundaries(boundaries, false), ":not([data-ht-each] *, [data-ht-with] *)"; got != want {
		t.Fatalf("boundary descendants selector = %q, want %q", got, want)
	}

	if got, want := (ApplyDirective{}).Selectors(true), []string{"[data-ht-apply]:not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *)"}; !slices.Equal(got, want) {
		t.Fatalf("default selectors = %v, want %v", got, want)
	}
	if got, want := (ApplyDirective{}).Selectors(false), []string{
		"[data-ht-apply]:not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *, [ht-each], [ht-each] *, [ht-with], [ht-with] *)",
		"[ht-apply]:not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *, [ht-each], [ht-each] *, [ht-with], [ht-with] *)",
	}; !slices.Equal(got, want) {
		t.Fatalf("legacy default selectors = %v, want %v", got, want)
	}
	if got, want := basedAttributeSelector("href", true), "[href]:not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *, [data-ht-base], [data-ht-base] *)"; got != want {
		t.Fatalf("strict based selector = %q, want %q", got, want)
	}
	if got, want := basedAttributeSelector("href", false), "[href]:not([data-ht-each], [data-ht-each] *, [data-ht-with], [data-ht-with] *, [data-ht-base], [data-ht-base] *, [ht-each], [ht-each] *, [ht-with], [ht-with] *, [ht-base], [ht-base] *)"; got != want {
		t.Fatalf("legacy based selector = %q, want %q", got, want)
	}
}

func TestBoundarySelectorsPreserveLegacyQueriesWithoutWithScopes(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<!doctype html><html><body>
		<div data-ht-apply id="apply-root"></div>
		<section data-ht-each="item in ${ items }"><div data-ht-apply id="apply-data-nested"></div><example-fragment data-ht-include="data.html"></example-fragment><a href="data.html"></a></section>
		<section ht-each="item in ${ items }"><div ht-apply id="apply-legacy-nested"></div><example-fragment ht-include="legacy.html"></example-fragment><a href="legacy.html"></a></section>
		<div ht-apply id="apply-root-legacy"></div><example-fragment data-ht-include="root.html"></example-fragment><example-fragment ht-include="root-legacy.html"></example-fragment><a href="root.html"></a>
	</body></html>`))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		queryAll bool
		before   []string
		current  []string
		after    []string
	}{
		{
			name: "default directive",
			before: []string{
				"[data-ht-apply]:not([data-ht-each] [data-ht-apply], [data-ht-each][data-ht-apply]):not([ht-each] [data-ht-apply], [ht-each][data-ht-apply])",
				"[ht-apply]:not([ht-each] [ht-apply], [ht-each][ht-apply]):not([data-ht-each] [ht-apply], [data-ht-each][ht-apply])",
			},
			current: []string{
				"[data-ht-apply]:not([data-ht-each][data-ht-apply], [data-ht-with][data-ht-apply], [ht-each][data-ht-apply], [ht-with][data-ht-apply]):not([data-ht-each] [data-ht-apply], [data-ht-with] [data-ht-apply], [ht-each] [data-ht-apply], [ht-with] [data-ht-apply])",
				"[ht-apply]:not([data-ht-each][ht-apply], [data-ht-with][ht-apply], [ht-each][ht-apply], [ht-with][ht-apply]):not([data-ht-each] [ht-apply], [data-ht-with] [ht-apply], [ht-each] [ht-apply], [ht-with] [ht-apply])",
			},
			after: (ApplyDirective{}).Selectors(false),
		},
		{
			name:     "include directive",
			queryAll: true,
			before: []string{
				"[data-ht-include]:not([data-ht-each] [data-ht-include]):not([ht-each] [data-ht-include])",
				"[ht-include]:not([ht-each] [ht-include]):not([data-ht-each] [ht-include])",
			},
			current: []string{
				"[data-ht-include]:not([data-ht-each] [data-ht-include], [data-ht-with] [data-ht-include], [ht-each] [data-ht-include], [ht-with] [data-ht-include])",
				"[ht-include]:not([data-ht-each] [ht-include], [data-ht-with] [ht-include], [ht-each] [ht-include], [ht-with] [ht-include])",
			},
			after: (IncludeDirective{}).Selectors(false),
		},
		{
			name: "each directive",
			before: []string{
				"[data-ht-each]:not([data-ht-each] [data-ht-each]):not([ht-each] [data-ht-each])",
				"[ht-each]:not([ht-each] [ht-each]):not([data-ht-each] [ht-each])",
			},
			current: []string{
				"[data-ht-each]:not([data-ht-each] [data-ht-each], [data-ht-with] [data-ht-each], [ht-each] [data-ht-each], [ht-with] [data-ht-each])",
				"[data-ht-with]:not([data-ht-each] [data-ht-with], [data-ht-with] [data-ht-with], [ht-each] [data-ht-with], [ht-with] [data-ht-with])",
				"[ht-each]:not([data-ht-each] [ht-each], [data-ht-with] [ht-each], [ht-each] [ht-each], [ht-with] [ht-each])",
				"[ht-with]:not([data-ht-each] [ht-with], [data-ht-with] [ht-with], [ht-each] [ht-with], [ht-with] [ht-with])",
			},
			after: (EachDirective{}).Selectors(false),
		},
		{
			name: "base URL resolution",
			before: []string{
				"[href]:not([data-ht-each] [href], [data-ht-each][href], [data-ht-base] [href], [data-ht-base][href]):not([ht-each] [href], [ht-each][href], [ht-base] [href], [ht-base][href])",
			},
			current: []string{
				"[href]:not([data-ht-each][href], [data-ht-with][href], [data-ht-base][href], [ht-each][href], [ht-with][href], [ht-base][href]):not([data-ht-each] [href], [data-ht-with] [href], [data-ht-base] [href], [ht-each] [href], [ht-with] [href], [ht-base] [href])",
			},
			after: []string{basedAttributeSelector("href", false)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := htdocument.QueryRoot
			if test.queryAll {
				query = htdocument.QueryAll
			}
			before := query(layout, test.before...)
			if got := query(layout, test.current...); !slices.Equal(got, before) {
				t.Fatalf("pre-wildcard query selected different elements")
			}
			if got := query(layout, test.after...); !slices.Equal(got, before) {
				t.Fatalf("wildcard query selected different elements")
			}
		})
	}
}

func TestConditionalElementsPreserveRootExclusionsAndDocumentOrder(t *testing.T) {
	root := &html.Node{
		Type: html.ElementNode,
		Data: "main",
		Attr: []html.Attribute{{Key: "data-ht-if", Val: "${ root }"}},
	}
	first := &html.Node{
		Type: html.ElementNode,
		Data: "span",
		Attr: []html.Attribute{{Key: "ht-if", Val: "${ first }"}},
	}
	templateRoot := &html.Node{
		Type: html.ElementNode,
		Data: "section",
		Attr: []html.Attribute{{Key: "data-ht-each", Val: "item in ${ items }"}},
	}
	templateChild := &html.Node{
		Type: html.ElementNode,
		Data: "p",
		Attr: []html.Attribute{{Key: "data-ht-if", Val: "${ deferred }"}},
	}
	last := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: []html.Attribute{{Key: "data-ht-if", Val: "${ last }"}},
	}
	root.AppendChild(first)
	root.AppendChild(templateRoot)
	templateRoot.AppendChild(templateChild)
	root.AppendChild(last)

	document := &htdocument.Document{Layout: root}
	elements := (ConditionalDirective{}).Elements(document)
	if len(elements) != 3 || elements[0] != root || elements[1] != first || elements[2] != last {
		t.Fatalf("elements = %v, want root, first, last in document order", elements)
	}

	document.StrictMode = true
	elements = (ConditionalDirective{}).Elements(document)
	if len(elements) != 2 || elements[0] != root || elements[1] != last {
		t.Fatalf("strict elements = %v, want root, last in document order", elements)
	}
}

var sharedSelectorAttributes = []string{"data-shared", "shared"}

type sharedSelectorDirective struct{}

func (sharedSelectorDirective) Attributes(strict bool) []string {
	if strict {
		return sharedSelectorAttributes[:1]
	}
	return sharedSelectorAttributes
}

func (sharedSelectorDirective) Selectors(strict bool) []string {
	return defaultDirectiveSelectors(sharedSelectorDirective{}, strict)
}

func (sharedSelectorDirective) Elements(*htdocument.Document) []*html.Node { return nil }

func (sharedSelectorDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (sharedSelectorDirective) Process(*htdocument.Document, *html.Node, *htdata.TemplateData, *htdata.TemplateData, io.Writer) error {
	return nil
}

func TestDefaultDirectiveSelectorsDoNotMutateSharedAttributes(t *testing.T) {
	_ = defaultDirectiveSelectors(sharedSelectorDirective{}, false)
	if got, want := sharedSelectorAttributes, []string{"data-shared", "shared"}; !slices.Equal(got, want) {
		t.Fatalf("shared attributes = %v, want %v", got, want)
	}
}
