package document

import (
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
