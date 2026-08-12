package directives

import (
	"strings"
	"testing"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
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
