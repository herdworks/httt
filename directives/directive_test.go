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
		directive Directive
		strict    []string
		legacy    []string
	}{
		{
			name:      "deprecated",
			directive: DeprecatedDirective{},
			strict:    []string{"[data-ht-not]"},
			legacy:    []string{"[data-ht-not]", "[ht-not]"},
		},
		{
			name:      "conditional",
			directive: ConditionalDirective{},
			strict:    []string{"[data-ht-if]:not([data-ht-template] [data-ht-if], [data-ht-template][data-ht-if])"},
			legacy: []string{
				"[data-ht-if]:not([data-ht-template] [data-ht-if], [data-ht-template][data-ht-if]):not([ht-template] [data-ht-if], [ht-template][data-ht-if])",
				"[ht-if]:not([ht-template] [ht-if], [ht-template][ht-if]):not([data-ht-template] [ht-if], [data-ht-template][ht-if])",
			},
		},
		{
			name:      "debug",
			directive: DebugDirective{},
			strict:    []string{"[data-ht-debug]:not([data-ht-template] [data-ht-debug], [data-ht-template][data-ht-debug])"},
			legacy: []string{
				"[data-ht-debug]:not([data-ht-template] [data-ht-debug], [data-ht-template][data-ht-debug]):not([ht-template] [data-ht-debug], [ht-template][data-ht-debug])",
				"[ht-debug]:not([ht-template] [ht-debug], [ht-template][ht-debug]):not([data-ht-template] [ht-debug], [data-ht-template][ht-debug])",
			},
		},
		{
			name:      "template",
			directive: TemplateDirective{},
			strict:    []string{"[data-ht-template]:not([data-ht-template] [data-ht-template])"},
			legacy: []string{
				"[data-ht-template]:not([data-ht-template] [data-ht-template]):not([ht-template] [data-ht-template])",
				"[ht-template]:not([ht-template] [ht-template]):not([data-ht-template] [ht-template])",
			},
		},
		{
			name:      "apply",
			directive: ApplyDirective{},
			strict: []string{
				"[data-ht-apply]:not([data-ht-template] [data-ht-apply], [data-ht-template][data-ht-apply])",
				"[data-hyper-template]:not([data-ht-template] [data-hyper-template], [data-ht-template][data-hyper-template])",
			},
			legacy: []string{
				"[data-ht-apply]:not([data-ht-template] [data-ht-apply], [data-ht-template][data-ht-apply]):not([ht-template] [data-ht-apply], [ht-template][data-ht-apply])",
				"[data-hyper-template]:not([data-ht-template] [data-hyper-template], [data-ht-template][data-hyper-template]):not([ht-template] [data-hyper-template], [ht-template][data-hyper-template])",
				"[ht-apply]:not([ht-template] [ht-apply], [ht-template][ht-apply]):not([data-ht-template] [ht-apply], [data-ht-template][ht-apply])",
				"[hyper-template]:not([ht-template] [hyper-template], [ht-template][hyper-template]):not([data-ht-template] [hyper-template], [data-ht-template][hyper-template])",
			},
		},
		{
			name:      "attrs",
			directive: AttrsDirective{},
			strict: []string{
				"[data-ht-attrs]:not([data-ht-template] [data-ht-attrs], [data-ht-template][data-ht-attrs])",
				"[data-ht-attr]:not([data-ht-template] [data-ht-attr], [data-ht-template][data-ht-attr])",
			},
			legacy: []string{
				"[data-ht-attrs]:not([data-ht-template] [data-ht-attrs], [data-ht-template][data-ht-attrs]):not([ht-template] [data-ht-attrs], [ht-template][data-ht-attrs])",
				"[data-ht-attr]:not([data-ht-template] [data-ht-attr], [data-ht-template][data-ht-attr]):not([ht-template] [data-ht-attr], [ht-template][data-ht-attr])",
				"[ht-attrs]:not([ht-template] [ht-attrs], [ht-template][ht-attrs]):not([data-ht-template] [ht-attrs], [data-ht-template][ht-attrs])",
				"[ht-attr]:not([ht-template] [ht-attr], [ht-template][ht-attr]):not([data-ht-template] [ht-attr], [data-ht-template][ht-attr])",
			},
		},
		{
			name:      "query",
			directive: QueryDirective{},
			strict:    []string{"[data-ht-query]:not([data-ht-template] [data-ht-query], [data-ht-template][data-ht-query])"},
			legacy: []string{
				"[data-ht-query]:not([data-ht-template] [data-ht-query], [data-ht-template][data-ht-query]):not([ht-template] [data-ht-query], [ht-template][data-ht-query])",
				"[ht-query]:not([ht-template] [ht-query], [ht-template][ht-query]):not([data-ht-template] [ht-query], [data-ht-template][ht-attrs])",
			},
		},
		{
			name:      "param",
			directive: ParamDirective{},
			strict:    []string{"[data-ht-param]:not([data-ht-template] [data-ht-param])"},
			legacy: []string{
				"[data-ht-param]:not([data-ht-template] [data-ht-param]):not([ht-template] [data-ht-param])",
				"[ht-param]:not([ht-template] [ht-param]):not([data-ht-template] [ht-param])",
			},
		},
		{
			name:      "content",
			directive: ContentDirective{},
			strict:    []string{"[data-ht-content]:not([data-ht-template] [data-ht-content])"},
			legacy: []string{
				"[data-ht-content]:not([data-ht-template] [data-ht-content]):not([ht-template] [data-ht-content])",
				"[ht-content]:not([ht-template] [ht-content]):not([data-ht-template] [ht-content])",
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
			strict:    []string{"[data-ht-pipe]:not([data-ht-template] [data-ht-pipe])"},
			legacy: []string{
				"[data-ht-pipe]:not([data-ht-template] [data-ht-pipe]):not([ht-template] [data-ht-pipe])",
				"[ht-pipe]:not([ht-template] [ht-pipe]):not([data-ht-template] [ht-pipe])",
			},
		},
		{
			name:      "base",
			directive: BaseDirective{},
			strict:    []string{"[data-ht-base]:not([data-ht-template] [data-ht-base], [data-ht-template][data-ht-base])"},
			legacy: []string{
				"[data-ht-base]:not([data-ht-template] [data-ht-base], [data-ht-template][data-ht-base]):not([ht-template] [data-ht-base], [ht-template][data-ht-base])",
				"[ht-base]:not([ht-template] [ht-base], [ht-template][ht-base]):not([data-ht-template] [ht-base], [data-ht-template][ht-base])",
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
		Attr: []html.Attribute{{Key: "data-ht-template", Val: "item:items"}},
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

func (sharedSelectorDirective) Process(*htdocument.Document, *html.Node, *htdata.TemplateData, *htdata.TemplateData, io.Writer) error {
	return nil
}

func TestDefaultDirectiveSelectorsDoNotMutateSharedAttributes(t *testing.T) {
	_ = defaultDirectiveSelectors(sharedSelectorDirective{}, false)
	if got, want := sharedSelectorAttributes, []string{"data-shared", "shared"}; !slices.Equal(got, want) {
		t.Fatalf("shared attributes = %v, want %v", got, want)
	}
}
