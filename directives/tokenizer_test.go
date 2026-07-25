package directives

import (
	"reflect"
	"testing"
)

func TestTokenizer(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		grammar    ExpressionGrammar
		want       []Token
		wantErr    bool
	}{
		{
			name:       "literals operator and variable",
			expression: "item in ${ page.foo, page.bar }",
			grammar:    ExpressionGrammar{Operators: []string{"in"}},
			want: []Token{
				{Kind: TokenLiteral, Raw: "item", Start: 0, End: 4},
				{Kind: TokenOperator, Raw: "in", Start: 5, End: 7},
				{Kind: TokenVariable, Raw: "${ page.foo, page.bar }", Start: 8, End: 31},
			},
		},
		{
			name:       "quoted literal",
			expression: `${ page.title } == "hello world"`,
			grammar:    ExpressionGrammar{Operators: []string{"=="}, AllowQuotedLiterals: true},
			want: []Token{
				{Kind: TokenVariable, Raw: `${ page.title }`, Start: 0, End: 15},
				{Kind: TokenOperator, Raw: "==", Start: 16, End: 18},
				{Kind: TokenLiteral, Raw: `"hello world"`, Quoted: true, Start: 19, End: 32},
			},
		},
		{
			name:       "longest multi word operator",
			expression: "value NOT\tIN ${ values }",
			grammar:    ExpressionGrammar{Operators: []string{"IN", "NOT IN"}},
			want: []Token{
				{Kind: TokenLiteral, Raw: "value", Start: 0, End: 5},
				{Kind: TokenOperator, Raw: "NOT\tIN", Start: 6, End: 12},
				{Kind: TokenVariable, Raw: "${ values }", Start: 13, End: 24},
			},
		},
		{
			name:       "symbolic operator adjacency",
			expression: `${ left }!="right"`,
			grammar:    ExpressionGrammar{Operators: []string{"!=", "!"}, AllowQuotedLiterals: true},
			want: []Token{
				{Kind: TokenVariable, Raw: `${ left }`, Start: 0, End: 9},
				{Kind: TokenOperator, Raw: "!=", Start: 9, End: 11},
				{Kind: TokenLiteral, Raw: `"right"`, Quoted: true, Start: 11, End: 18},
			},
		},
		{name: "operator case is exact", expression: "item IN ${ items }", grammar: ExpressionGrammar{Operators: []string{"in"}}, want: []Token{{Kind: TokenLiteral, Raw: "item", Start: 0, End: 4}, {Kind: TokenLiteral, Raw: "IN", Start: 5, End: 7}, {Kind: TokenVariable, Raw: "${ items }", Start: 8, End: 18}}},
		{name: "disabled quoted literal", expression: `"value"`, grammar: ExpressionGrammar{}, wantErr: true},
		{name: "unterminated quoted literal", expression: `"value`, grammar: ExpressionGrammar{AllowQuotedLiterals: true}, wantErr: true},
		{name: "unterminated variable", expression: `${ value`, grammar: ExpressionGrammar{}, wantErr: true},
		{name: "ambiguous grammar", expression: "value", grammar: ExpressionGrammar{Operators: []string{"in", "in"}}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Tokenizer(test.expression, test.grammar)
			if test.wantErr {
				if err == nil {
					t.Fatal("Tokenizer() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Tokenizer() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestTokenizerKeepsQuotedVariableContentRaw(t *testing.T) {
	expression := `item in ${ get(page.title, "\"}") }`
	tokens, err := Tokenizer(expression, ExpressionGrammar{Operators: []string{"in"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 || tokens[2].Kind != TokenVariable || tokens[2].Raw != `${ get(page.title, "\"}") }` {
		t.Fatalf("Tokenizer() = %#v", tokens)
	}
}
