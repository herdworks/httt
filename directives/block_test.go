package directives

import (
	"bytes"
	"strings"
	"testing"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
)

func TestBlockRunsOnlyInTopLevelDocument(t *testing.T) {
	document := setupBuiltinTest(t, `<!doctype html><html><body><outer-block ht-block></outer-block></body></html>`, map[string]string{
		"blocks/outer-block.html": `<outer-block><inner-block ht-block></inner-block></outer-block>`,
		"blocks/inner-block.html": `<inner-block data-rendered="true"></inner-block>`,
	})
	document.TidyMode = true

	output, err := Process(document, &htdata.TemplateData{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(output, []byte(`data-rendered="true"`)) {
		t.Fatalf("nested block rendered from a block fragment:\n%s", output)
	}
	if !bytes.Contains(output, []byte(`<inner-block ht-block=""></inner-block>`)) {
		t.Fatalf("expected nested block to remain inert:\n%s", output)
	}

	fragment := &htdocument.Document{Layout: document.Layout.FirstChild}
	if elements := (BlockDirective{}).Elements(fragment); len(elements) != 0 {
		t.Fatalf("fragment block elements = %d, want 0", len(elements))
	}
}

func TestBlockUsesOnlySafeUserDataAndBlockData(t *testing.T) {
	document := setupBuiltinTest(t, `<!doctype html><html><body><section ht-apply>${ page.content }</section></body></html>`, map[string]string{
		"blocks/safe-block.html": `<safe-block ht-apply data-page="${ page.title }" data-block="${ block.value }">${ block.content }<meta name="layout:secret" content="yes"><span ht-if="${ outer.secret }">outer leak</span><span ht-if="${ layout.secret }">layout leak</span></safe-block>`,
	})
	document.TidyMode = true
	data := htdata.TemplateData{
		"outer": htdata.TemplateData{"secret": true},
		"page": htdata.TemplateData{
			"content": `<safe-block ht-block value="attribute">child content</safe-block>`,
			"title":   "outer page",
		},
	}
	userData := htdata.TemplateData{"page": htdata.TemplateData{"title": "safe page"}}

	output, err := Process(document, &data, &userData, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`data-page="safe page"`, `data-block="attribute"`, `child content`} {
		if !bytes.Contains(output, []byte(want)) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
	if bytes.Contains(output, []byte("outer page")) || bytes.Contains(output, []byte("outer leak")) || bytes.Contains(output, []byte("layout leak")) {
		t.Fatalf("block read ordinary render data:\n%s", output)
	}
}

func TestBlockWithNilUserDataHasOnlyBlockData(t *testing.T) {
	document := setupBuiltinTest(t, `<!doctype html><html><body><safe-block ht-block value="attribute">child content</safe-block></body></html>`, map[string]string{
		"blocks/safe-block.html": `<safe-block ht-apply data-block="${ block.value }">${ block.content }<span ht-if="${ outer.secret }">leaked</span></safe-block>`,
	})
	document.TidyMode = true
	data := htdata.TemplateData{"outer": htdata.TemplateData{"secret": true}}

	output, err := Process(document, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`data-block="attribute"`)) || !bytes.Contains(output, []byte("child content")) {
		t.Fatalf("block data missing:\n%s", output)
	}
	if bytes.Contains(output, []byte("leaked")) {
		t.Fatalf("block read ordinary render data:\n%s", output)
	}
}

func TestBlockPassAfterEachDoesNotExposeBinding(t *testing.T) {
	document := setupBuiltinTest(t, `<!doctype html><html><body><div ht-each="item in ${ items }"><loop-block ht-block></loop-block></div></body></html>`, map[string]string{
		"blocks/loop-block.html": `<loop-block ht-apply data-index='${ each.index, "no-loop" }'></loop-block>`,
	})
	document.TidyMode = true
	data := htdata.TemplateData{"items": []string{"one", "two"}}

	output, err := Process(document, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(output), `data-index="no-loop"`); got != 2 {
		t.Fatalf("root block pass rendered %d loop blocks without the binding, want 2:\n%s", got, output)
	}
}
