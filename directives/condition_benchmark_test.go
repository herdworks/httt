package directives

import (
	"errors"
	"strings"
	"testing"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func BenchmarkConditionalDirectiveProcess(b *testing.B) {
	tags := []string{"draft", "post", "note"}
	cases := []struct {
		name       string
		expression string
		data       htdata.TemplateData
	}{
		{name: "short_presence", expression: "${ page.published }", data: htdata.TemplateData{"page.published": true}},
		{name: "or_first_true", expression: "${ first } OR ${ middle } OR ${ last }", data: htdata.TemplateData{"first": true, "middle": true, "last": true}},
		{name: "or_middle_true", expression: "${ first } OR ${ middle } OR ${ last }", data: htdata.TemplateData{"first": false, "middle": true, "last": true}},
		{name: "or_last_true", expression: "${ first } OR ${ middle } OR ${ last }", data: htdata.TemplateData{"first": false, "middle": false, "last": true}},
		{name: "or_all_false", expression: "${ first } OR ${ middle } OR ${ last }", data: htdata.TemplateData{"first": false, "middle": false, "last": false}},
		{name: "quoted_literal", expression: `${ page.kind } == "post"`, data: htdata.TemplateData{"page.kind": "post"}},
		{name: "symbolic_comparison", expression: `${ page.kind } != "draft"`, data: htdata.TemplateData{"page.kind": "post"}},
		{name: "membership", expression: `"post" in ${ page.tags }`, data: htdata.TemplateData{"page.tags": tags}},
		{name: "regex", expression: `${ page.path } =~ "^/blog/"`, data: htdata.TemplateData{"page.path": "/blog/one"}},
		{name: "long_or_last_true", expression: `${ one } OR ${ two } OR ${ three } OR ${ four } OR ${ five } OR ${ six } OR ${ seven } OR ${ eight }`, data: htdata.TemplateData{"one": false, "two": false, "three": false, "four": false, "five": false, "six": false, "seven": false, "eight": true}},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			element := &html.Node{Type: html.ElementNode, Data: "div"}
			document := &htdocument.Document{Layout: element}
			directive := ConditionalDirective{}
			b.ReportAllocs()
			for b.Loop() {
				element.Attr = []html.Attribute{{Key: "ht-if", Val: test.expression}}
				if err := directive.Process(document, element, &test.data, nil, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkConditionalDirectiveProcessIncremental(b *testing.B) {
	tags := []string{"draft", "post", "note"}
	cases := []struct {
		name       string
		expression string
		data       htdata.TemplateData
	}{
		{name: "short_presence", expression: "${ page.published }", data: htdata.TemplateData{"page.published": true}},
		{name: "or_first_true", expression: "${ first } OR ${ middle } OR ${ last }", data: htdata.TemplateData{"first": true, "middle": true, "last": true}},
		{name: "or_middle_true", expression: "${ first } OR ${ middle } OR ${ last }", data: htdata.TemplateData{"first": false, "middle": true, "last": true}},
		{name: "or_last_true", expression: "${ first } OR ${ middle } OR ${ last }", data: htdata.TemplateData{"first": false, "middle": false, "last": true}},
		{name: "or_all_false", expression: "${ first } OR ${ middle } OR ${ last }", data: htdata.TemplateData{"first": false, "middle": false, "last": false}},
		{name: "quoted_literal", expression: `${ page.kind } == "post"`, data: htdata.TemplateData{"page.kind": "post"}},
		{name: "symbolic_comparison", expression: `${ page.kind } != "draft"`, data: htdata.TemplateData{"page.kind": "post"}},
		{name: "membership", expression: `"post" in ${ page.tags }`, data: htdata.TemplateData{"page.tags": tags}},
		{name: "regex", expression: `${ page.path } =~ "^/blog/"`, data: htdata.TemplateData{"page.path": "/blog/one"}},
		{name: "long_or_last_true", expression: `${ one } OR ${ two } OR ${ three } OR ${ four } OR ${ five } OR ${ six } OR ${ seven } OR ${ eight }`, data: htdata.TemplateData{"one": false, "two": false, "three": false, "four": false, "five": false, "six": false, "seven": false, "eight": true}},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			element := &html.Node{Type: html.ElementNode, Data: "div"}
			document := &htdocument.Document{Layout: element}
			b.ReportAllocs()
			for b.Loop() {
				element.Attr = []html.Attribute{{Key: "ht-if", Val: test.expression}}
				if err := processConditionalDirectiveIncremental(document, element, &test.data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func processConditionalDirectiveIncremental(document *htdocument.Document, element *html.Node, data *htdata.TemplateData) error {
	attributes := htdocument.GetAttributes(element, ConditionalDirective{}.Attributes(document.StrictMode)...)
	funcs := document.Plugins.FuncMap()
	for _, attribute := range attributes {
		if strings.TrimSpace(attribute.Val) == "" {
			return errors.New("ht-if directives require a non-empty conditional expression")
		}
		var expression conditionalExpression
		if err := parseConditionalExpressionIncremental(attribute.Val, &expression); err != nil {
			return err
		}
		matched := false
		for index := 0; index < expression.count; index++ {
			truthy, err := evaluateConditionalStatement(expression.at(index), data, funcs)
			if err != nil {
				return err
			}
			if truthy {
				matched = true
				break
			}
		}
		if !matched {
			htdocument.SetAttribute(element, "hidden", "true")
			if document.TidyMode {
				htdocument.RemoveElement(element)
			}
			return nil
		}
	}
	if !document.TidyMode {
		htdocument.RemoveAttributes(element, "hidden")
	}
	return nil
}

func BenchmarkConditionalSyntaxTree(b *testing.B) {
	source := `${ first } OR ${ middle } OR ${ last }`
	b.ReportAllocs()
	for b.Loop() {
		var expression conditionalExpression
		if err := parseConditionalExpression(source, &expression); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConditionalIncrementalSyntaxTree(b *testing.B) {
	source := `${ first } OR ${ middle } OR ${ last }`
	b.ReportAllocs()
	for b.Loop() {
		var expression conditionalExpression
		if err := parseConditionalExpressionIncremental(source, &expression); err != nil {
			b.Fatal(err)
		}
	}
}

// parseConditionalExpressionIncremental is a benchmark candidate for callers
// that choose the optional delimiter-bounded API. It deliberately parses and
// validates every branch before returning, matching ht-if's eager syntax
// contract; it is not the ConditionalDirective runtime path.
func parseConditionalExpressionIncremental(source string, expression *conditionalExpression) error {
	directive := ConditionalDirective{}
	for remainder := source; ; {
		next, tokens, err := directive.ParseNextExpression(remainder)
		if err != nil {
			return err
		}
		if err := directive.ValidateExpression(tokens); err != nil {
			return err
		}
		statement, err := parseConditionalTokenGroup(remainder, tokens)
		if err != nil {
			return err
		}
		expression.append(statement)
		if next == "" {
			return nil
		}
		remainder = next
	}
}

func BenchmarkConditionalTokenStream(b *testing.B) {
	source := `${ first } OR ${ middle } OR ${ last }`
	b.ReportAllocs()
	for b.Loop() {
		if _, err := tokenizeExpression(source, conditionalExpressionGrammar); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHTMLFragmentTree(b *testing.B) {
	source := `<article ht-if='${ first } OR ${ middle } OR ${ last }'><span>example</span></article>`
	context := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := html.ParseFragment(strings.NewReader(source), context); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenericTokenizer(b *testing.B) {
	cases := []struct {
		name       string
		expression string
		grammar    ExpressionGrammar
	}{
		{name: "conditional", expression: `${ page.kind } != "draft" OR ${ page.path } =~ "^/blog/"`, grammar: conditionalExpressionGrammar},
		{name: "each", expression: `post in ${ posts }`, grammar: eachExpressionGrammar},
		{name: "base", expression: `href on ${ site.url }`, grammar: baseExpressionGrammar},
		{name: "attrs", expression: `${ page.attributes }`, grammar: attributeMapExpressionGrammar},
	}

	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := tokenizeExpression(test.expression, test.grammar); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkTokenizerMaxTokens(b *testing.B) {
	cases := []struct {
		name       string
		expression string
		grammar    ExpressionGrammar
	}{
		{name: "each_unbounded", expression: `post in ${ posts }`, grammar: ExpressionGrammar{Operators: []string{"in"}}},
		{name: "each_bounded", expression: `post in ${ posts }`, grammar: eachExpressionGrammar},
		{name: "base_unbounded", expression: `href on ${ site.url }`, grammar: ExpressionGrammar{Operators: []string{"on"}}},
		{name: "base_bounded", expression: `href on ${ site.url }`, grammar: baseExpressionGrammar},
		{name: "attrs_unbounded", expression: `${ page.attributes }`, grammar: ExpressionGrammar{}},
		{name: "attrs_bounded", expression: `${ page.attributes }`, grammar: attributeMapExpressionGrammar},
		{name: "conditional_unbounded", expression: `${ first } OR ${ middle } OR ${ last }`, grammar: ExpressionGrammar{Separator: "OR", AllowQuotedLiterals: true, Operators: conditionalExpressionGrammar.Operators}},
		{name: "conditional_bounded", expression: `${ first } OR ${ middle } OR ${ last }`, grammar: conditionalExpressionGrammar},
	}
	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := tokenizeExpression(test.expression, test.grammar); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
