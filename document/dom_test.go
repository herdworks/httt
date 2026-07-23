package document

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestCloneElementPreservesNamespace(t *testing.T) {
	element := &html.Node{
		Type:      html.ElementNode,
		Data:      "svg",
		Namespace: "svg",
		Attr:      []html.Attribute{{Namespace: "xlink", Key: "href", Val: "#icon"}},
	}
	element.AppendChild(&html.Node{Type: html.ElementNode, Data: "use", Namespace: "svg"})

	clone := CloneElement(element)
	if clone == element || clone.Namespace != "svg" || clone.FirstChild == element.FirstChild || clone.FirstChild.Namespace != "svg" {
		t.Fatalf("clone = %#v", clone)
	}
	if len(clone.Attr) != 1 || clone.Attr[0].Namespace != "xlink" {
		t.Fatalf("clone attrs = %#v", clone.Attr)
	}
}

func TestQueryHelpersTreatVariadicSelectorsAsDocumentOrderOR(t *testing.T) {
	root, err := html.Parse(strings.NewReader(`<main><span id="first"></span><p id="middle"></p><span id="last"></span></main>`))
	if err != nil {
		t.Fatal(err)
	}

	if got := Query(root, "span#last", "p#middle"); got == nil || got.Data != "p" {
		t.Fatalf("Query returned %v, want p#middle", got)
	}

	all := QueryAll(root, "span", "p", "span#first")
	if len(all) != 3 || all[0].Attr[0].Val != "first" || all[1].Data != "p" || all[2].Attr[0].Val != "last" {
		t.Fatalf("QueryAll order = %#v, want first, middle, last", all)
	}
}

func TestQueryRootTreatsVariadicSelectorsAsDocumentOrderORAndIncludesRoot(t *testing.T) {
	root, err := html.Parse(strings.NewReader(`<main class="target"><span id="child"></span></main>`))
	if err != nil {
		t.Fatal(err)
	}

	htmlRoot := root.FirstChild
	all := QueryRoot(root, "html", "main", "main.target")
	if len(all) != 2 || all[0].Data != "html" || all[1].Data != "main" {
		t.Fatalf("QueryRoot matches = %#v, want html then main once", all)
	}

	all = QueryRoot(htmlRoot, "html", "main.target", "span")
	if len(all) != 3 || all[0].Data != "html" || all[1].Data != "main" || all[2].Data != "span" {
		t.Fatalf("QueryRoot root matches = %#v, want html, main, span", all)
	}
}

func TestGetAttributeMatchesCaseInsensitively(t *testing.T) {
	element := &html.Node{
		Type: html.ElementNode,
		Attr: []html.Attribute{
			{Key: "HT-IF", Val: "first"},
			{Key: "data-ht-if", Val: "second"},
			{Key: "class", Val: "card"},
		},
	}

	if got := GetAttribute(element, "ht-if"); got.Key != "HT-IF" || got.Val != "first" {
		t.Fatalf("GetAttribute = %#v, want original first match", got)
	}
}

func TestGetAttributesPreservesSourceOrderAndOriginalAttributes(t *testing.T) {
	element := &html.Node{
		Type: html.ElementNode,
		Attr: []html.Attribute{
			{Key: "class", Val: "card"},
			{Key: "HT-IF", Val: "first"},
			{Key: "data-ht-if", Val: "second"},
		},
	}

	got := GetAttributes(element, "data-ht-if", "ht-if")
	if len(got) != 2 || got[0].Key != "HT-IF" || got[0].Val != "first" || got[1].Key != "data-ht-if" || got[1].Val != "second" {
		t.Fatalf("GetAttributes = %#v, want matching attributes in source order", got)
	}
}

func TestGetAttributeHelpersHandleMissingAndNonElementNodes(t *testing.T) {
	if got := GetAttribute(nil, "data-ht-if"); got != (html.Attribute{}) {
		t.Fatalf("GetAttribute(nil) = %#v, want zero attribute", got)
	}
	if got := GetAttributes(nil, "data-ht-if"); got != nil {
		t.Fatalf("GetAttributes(nil) = %#v, want nil", got)
	}

	text := &html.Node{Type: html.TextNode, Data: "text"}
	if got := GetAttribute(text, "data-ht-if"); got != (html.Attribute{}) {
		t.Fatalf("GetAttribute(text) = %#v, want zero attribute", got)
	}
	if got := GetAttributes(text, "data-ht-if"); got != nil {
		t.Fatalf("GetAttributes(text) = %#v, want nil", got)
	}

	element := &html.Node{Type: html.ElementNode, Attr: []html.Attribute{{Key: "class", Val: "card"}}}
	if got := GetAttribute(element, "data-ht-if"); got != (html.Attribute{}) {
		t.Fatalf("GetAttribute(missing) = %#v, want zero attribute", got)
	}
	if got := GetAttributes(element, "data-ht-if"); got != nil {
		t.Fatalf("GetAttributes(missing) = %#v, want nil", got)
	}
}
