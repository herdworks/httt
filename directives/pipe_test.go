package directives

import (
	"strings"
	"testing"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

func TestPipeParsesQuotedDestinationsAndTypes(t *testing.T) {
	directive := PipeDirective{}
	for _, source := range []string{
		`to "main"`,
		`to "head style#components" as css`,
		`to "main" as "element"`,
		`to '${ layout.pipe_target }' as javascript`,
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
		`main`, `to main`, `to "main" css`, `to "main" as unknown`, `to "main`, `to ""`,
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
	tokens, err := directive.ParseExpression(`to "main" as`)
	if err != nil {
		t.Fatal(err)
	}
	if err := directive.ValidateExpression(tokens); err == nil || err.Error() != `ht-pipe: syntax error (expected 'to "<selector>"' or 'to "<selector>" as <type>'; got: "to \"main\" as")` {
		t.Fatalf("ValidateExpression() error = %v", err)
	}
}

func TestPipeMovesElementsAndAppendsText(t *testing.T) {
	t.Run("element", func(t *testing.T) {
		document, element := pipeDocument(t, `<html><body><main id="target"></main><aside ht-pipe='to "#target"'><p>Moved</p></aside></body></html>`)
		if err := (PipeDirective{}).Process(document, element, &htdata.TemplateData{}, nil, nil); err != nil {
			t.Fatal(err)
		}
		target := htdocument.Query(document.Layout, "#target")
		if target == nil || target.FirstChild == nil || target.FirstChild.Data != "aside" {
			t.Fatalf("target = %s, want moved aside", htdocument.ElementString(target))
		}
	})
	t.Run("text aliases", func(t *testing.T) {
		document, element := pipeDocument(t, `<html><head><style id="target"></style></head><body><style ht-pipe='to "#target" as css'>body { color: red; }</style></body></html>`)
		if err := (PipeDirective{}).Process(document, element, &htdata.TemplateData{}, nil, nil); err != nil {
			t.Fatal(err)
		}
		target := htdocument.Query(document.Layout, "#target")
		if target == nil || !strings.Contains(htdocument.TextContent(target, true), "color: red") || element.Parent != nil {
			t.Fatalf("target = %s, source parent = %v", htdocument.ElementString(target), element.Parent)
		}
	})
}

func TestPipeTreatsTemplateSyntaxAsALiteralSelector(t *testing.T) {
	document, element := pipeDocument(t, `<html><body><main id="target"></main><aside ht-pipe='to "${ layout.pipe_target }"'></aside></body></html>`)
	data := htdata.TemplateData{"layout": htdata.TemplateData{"pipe_target": "#target"}}
	if err := (PipeDirective{}).Process(document, element, &data, nil, nil); err != nil {
		t.Fatal(err)
	}
	if element.Parent == nil {
		t.Fatal("literal selector was resolved as a template-data keypath")
	}
}

func TestPipeUsesSourceOrderedFallbackAttributesAndAllowsNoMatch(t *testing.T) {
	document, element := pipeDocument(t, `<html><body><main id="first"></main><main id="second"></main><aside ht-pipe='to "#missing"' data-ht-pipe='to "#second"'></aside></body></html>`)
	if err := (PipeDirective{}).Process(document, element, &htdata.TemplateData{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if target := htdocument.Query(document.Layout, "#second"); target == nil || target.FirstChild != element {
		t.Fatal("second pipe attribute did not receive the element after the first selector missed")
	}

	document, element = pipeDocument(t, `<html><body><aside ht-pipe='to "missing"'></aside></body></html>`)
	if err := (PipeDirective{}).Process(document, element, &htdata.TemplateData{}, nil, nil); err != nil {
		t.Fatalf("no-match pipe: %v", err)
	}
	if element.Parent == nil {
		t.Fatal("no-match pipe removed the source element")
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
		t.Fatal("missing pipe element")
	}
	return &htdocument.Document{Layout: layout}, element
}
