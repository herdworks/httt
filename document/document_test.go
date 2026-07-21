package document

import (
	"testing"

	"golang.org/x/net/html"
)

func TestDocumentIsFragment(t *testing.T) {
	tests := []struct {
		name string
		doc  *Document
		want bool
	}{
		{name: "nil document", doc: nil, want: false},
		{name: "nil layout", doc: &Document{}, want: false},
		{name: "complete document", doc: &Document{Layout: &html.Node{Type: html.DocumentNode}}, want: false},
		{name: "element fragment", doc: &Document{Layout: &html.Node{Type: html.ElementNode, Data: "section"}}, want: true},
		{name: "text fragment", doc: &Document{Layout: &html.Node{Type: html.TextNode, Data: "hello"}}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.doc.IsFragment(); got != test.want {
				t.Fatalf("IsFragment() = %v, want %v", got, test.want)
			}
		})
	}
}
