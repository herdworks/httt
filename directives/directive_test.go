package directives

import (
	"io"
	"slices"
	"testing"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

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
			name:      "conditional",
			directive: ConditionalDirective{},
			strict:    []string{"[data-ht-if]:not([data-ht-each] [data-ht-if], [data-ht-each][data-ht-if])"},
			legacy: []string{
				"[data-ht-if]:not([data-ht-each] [data-ht-if], [data-ht-each][data-ht-if]):not([ht-each] [data-ht-if], [ht-each][data-ht-if])",
				"[ht-if]:not([ht-each] [ht-if], [ht-each][ht-if]):not([data-ht-each] [ht-if], [data-ht-each][ht-if])",
			},
		},
		{
			name:      "debug",
			directive: DebugDirective{},
			strict:    []string{"[data-ht-debug]:not([data-ht-each] [data-ht-debug], [data-ht-each][data-ht-debug])"},
			legacy: []string{
				"[data-ht-debug]:not([data-ht-each] [data-ht-debug], [data-ht-each][data-ht-debug]):not([ht-each] [data-ht-debug], [ht-each][data-ht-debug])",
				"[ht-debug]:not([ht-each] [ht-debug], [ht-each][ht-debug]):not([data-ht-each] [ht-debug], [data-ht-each][ht-debug])",
			},
		},
		{
			name:      "include",
			directive: IncludeDirective{},
			strict:    []string{"[data-ht-include]:not([data-ht-each] [data-ht-include])"},
			legacy: []string{
				"[data-ht-include]:not([data-ht-each] [data-ht-include]):not([ht-each] [data-ht-include])",
				"[ht-include]:not([ht-each] [ht-include]):not([data-ht-each] [ht-include])",
			},
		},
		{
			name:      "each",
			directive: EachDirective{},
			strict:    []string{"[data-ht-each]:not([data-ht-each] [data-ht-each])"},
			legacy: []string{
				"[data-ht-each]:not([data-ht-each] [data-ht-each]):not([ht-each] [data-ht-each])",
				"[ht-each]:not([ht-each] [ht-each]):not([data-ht-each] [ht-each])",
			},
		},
		{
			name:      "apply",
			directive: ApplyDirective{},
			strict: []string{
				"[data-ht-apply]:not([data-ht-each] [data-ht-apply], [data-ht-each][data-ht-apply])",
				"[data-ht-with]:not([data-ht-each] [data-ht-with], [data-ht-each][data-ht-with])",
			},
			legacy: []string{
				"[data-ht-apply]:not([data-ht-each] [data-ht-apply], [data-ht-each][data-ht-apply]):not([ht-each] [data-ht-apply], [ht-each][data-ht-apply])",
				"[data-ht-with]:not([data-ht-each] [data-ht-with], [data-ht-each][data-ht-with]):not([ht-each] [data-ht-with], [ht-each][data-ht-with])",
				"[ht-apply]:not([ht-each] [ht-apply], [ht-each][ht-apply]):not([data-ht-each] [ht-apply], [data-ht-each][ht-apply])",
				"[ht-with]:not([ht-each] [ht-with], [ht-each][ht-with]):not([data-ht-each] [ht-with], [data-ht-each][ht-with])",
			},
		},
		{
			name:      "with",
			directive: WithModifier{},
			strict: []string{
				"[data-ht-with]:not([data-ht-each] [data-ht-with], [data-ht-each][data-ht-with])",
			},
			legacy: []string{
				"[data-ht-with]:not([data-ht-each] [data-ht-with], [data-ht-each][data-ht-with]):not([ht-each] [data-ht-with], [ht-each][data-ht-with])",
				"[ht-with]:not([ht-each] [ht-with], [ht-each][ht-with]):not([data-ht-each] [ht-with], [data-ht-each][ht-with])",
			},
		},
		{
			name:      "attrs",
			directive: AttributeMapDirective{},
			strict: []string{
				"[data-ht-attrs]:not([data-ht-each] [data-ht-attrs], [data-ht-each][data-ht-attrs])",
			},
			legacy: []string{
				"[data-ht-attrs]:not([data-ht-each] [data-ht-attrs], [data-ht-each][data-ht-attrs]):not([ht-each] [data-ht-attrs], [ht-each][data-ht-attrs])",
				"[ht-attrs]:not([ht-each] [ht-attrs], [ht-each][ht-attrs]):not([data-ht-each] [ht-attrs], [data-ht-each][ht-attrs])",
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
			strict:    []string{"[data-ht-base]:not([data-ht-each] [data-ht-base], [data-ht-each][data-ht-base])"},
			legacy: []string{
				"[data-ht-base]:not([data-ht-each] [data-ht-base], [data-ht-each][data-ht-base]):not([ht-each] [data-ht-base], [ht-each][data-ht-base])",
				"[ht-base]:not([ht-each] [ht-base], [ht-each][ht-base]):not([data-ht-each] [ht-base], [data-ht-each][ht-base])",
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
