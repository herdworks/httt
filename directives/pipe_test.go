package directives

import (
	"strings"
	"testing"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

func TestPipeParsesQuotedSourcesAndTypes(t *testing.T) {
	directive := PipeDirective{}
	for _, source := range []string{
		`from "main"`,
		`from "head style#components" as css`,
		`from "main" as "element"`,
		`from '${ layout.pipe_source }' as javascript`,
	} {
		tokens, err := directive.ParseExpression(source)
		if err != nil {
			t.Fatalf("ParseExpression(%q): %v", source, err)
		}
		if err := directive.ValidateExpression(tokens); err != nil {
			t.Fatalf("ValidateExpression(%q): %v", source, err)
		}
	}
	for _, source := range []string{
		`main`, `from main`, `to "main"`, `from "main" css`, `from "main" as unknown`, `from "main`, `from ""`,
	} {
		tokens, err := directive.ParseExpression(source)
		if err == nil {
			err = directive.ValidateExpression(tokens)
		}
		if err == nil {
			t.Fatalf("expression %q succeeded, want error", source)
		}
	}
}

func TestPipeReportsMalformedTokenCountsWithReconstructedSource(t *testing.T) {
	directive := PipeDirective{}
	tokens, err := directive.ParseExpression(`from "main" as`)
	if err != nil {
		t.Fatal(err)
	}
	if err := directive.ValidateExpression(tokens); err == nil || err.Error() != `ht-pipe: syntax error (expected 'from "<selector>"' or 'from "<selector>" as <type>'; got: "from \"main\" as")` {
		t.Fatalf("ValidateExpression() error = %v", err)
	}
}

func TestPipePullsElementsAndTextInSourceOrder(t *testing.T) {
	document, destination := pipeDocument(t, `<html><head><style id="styles" ht-pipe='from "style.component" as css' ht-pipe='from "script.component" as js'></style></head><body><style class="component">first { color: red; }</style><script class="component">second()</script></body></html>`)
	if err := (PipeDirective{}).Process(document, destination, &htdata.TemplateData{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := htdocument.TextContent(destination, true); !strings.Contains(got, "first { color: red; }") || !strings.Contains(got, "second()") || strings.Index(got, "first") > strings.Index(got, "second") {
		t.Fatalf("destination text = %q, want source-ordered collected content", got)
	}
	if htdocument.Query(document.Layout, "style.component") != nil || htdocument.Query(document.Layout, "script.component") != nil {
		t.Fatalf("pipe sources remained in document: %s", htdocument.ElementString(document.Layout))
	}
}

func TestPipeDeduplicatesAndRemovesAllMatchingElements(t *testing.T) {
	document, destination := pipeDocument(t, `<html><body><main id="target" ht-pipe='from ".component"'></main><aside class="component"><p>one</p></aside><aside class="component"><p>one</p></aside><aside class="component"><p>two</p></aside></body></html>`)
	if err := (PipeDirective{}).Process(document, destination, &htdata.TemplateData{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(htdocument.TextContent(destination, true), ">one<"); got != 1 {
		t.Fatalf("deduplicated destination contains %d copies of one", got)
	}
	if got := strings.Count(htdocument.TextContent(destination, true), ">two<"); got != 1 {
		t.Fatalf("destination contains %d copies of two", got)
	}
	components := htdocument.QueryAll(document.Layout, ".component")
	if len(components) != 2 || components[0].Parent != destination || components[1].Parent != destination {
		t.Fatalf("matching elements = %#v, want two deduplicated children", components)
	}
}

func TestPipeLeavesLiteralTemplateSyntaxUnresolvedAndSkipsFragments(t *testing.T) {
	document, destination := pipeDocument(t, `<html><body><main ht-pipe='from "${ layout.pipe_source }"'></main><aside class="component">source</aside></body></html>`)
	data := htdata.TemplateData{"layout": htdata.TemplateData{"pipe_source": ".component"}}
	if err := (PipeDirective{}).Process(document, destination, &data, nil, nil); err != nil {
		t.Fatal(err)
	}
	if htdocument.Query(document.Layout, ".component") == nil {
		t.Fatal("literal selector was resolved as a template-data keypath")
	}

	fragment := &htdocument.Document{Layout: destination}
	if elements := (PipeDirective{}).Elements(fragment); len(elements) != 0 {
		t.Fatalf("fragment pipe elements = %d, want 0", len(elements))
	}
	if err := (PipeDirective{}).Process(fragment, destination, &data, nil, nil); err != nil {
		t.Fatal(err)
	}
	if htdocument.Query(document.Layout, ".component") == nil {
		t.Fatal("fragment pipe processing removed source")
	}
}

func TestPipeRunsAfterEach(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><section ht-each='item in ${ items }'><aside class="component" ht-apply>${ item }</aside></section><main id="target" ht-pipe='from ".component"'></main></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	output, err := Process(&htdocument.Document{Layout: layout, TidyMode: true}, &htdata.TemplateData{"items": []string{"first", "second"}}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := string(output)
	if strings.Count(result, `class="component"`) != 2 || strings.Index(result, "first") > strings.Index(result, "second") || strings.Contains(result, "ht-pipe") {
		t.Fatalf("pipe did not collect loop output in source order:\n%s", result)
	}
}

func TestPipeRunsAfterBlock(t *testing.T) {
	document := setupBuiltinTest(t, `<!doctype html><html><head><style id="target" ht-pipe='from "style.component" as css'></style></head><body><component-block ht-block></component-block></body></html>`, map[string]string{
		"blocks/component-block.html": `<component-block><style class="component">component { color: red; }</style></component-block>`,
	})
	document.TidyMode = true
	output, err := Process(document, &htdata.TemplateData{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	result := string(output)
	if !strings.Contains(result, `id="target">component { color: red; }</style>`) || strings.Contains(result, `class="component"`) {
		t.Fatalf("pipe did not collect block output:\n%s", result)
	}
}

func pipeDocument(t *testing.T, source string) (*htdocument.Document, *html.Node) {
	t.Helper()
	layout, err := html.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	element := htdocument.Query(layout, "[ht-pipe]")
	if element == nil {
		t.Fatal("missing pipe destination")
	}
	return &htdocument.Document{Layout: layout}, element
}
