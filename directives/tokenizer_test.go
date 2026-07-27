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
		{name: "negative maximum tokens", expression: "value", grammar: ExpressionGrammar{MaxTokens: -1}, wantErr: true},
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

func TestTokenUnquoted(t *testing.T) {
	tests := []struct {
		token Token
		want  string
	}{
		{token: Token{Raw: `"head style"`, Quoted: true}, want: "head style"},
		{token: Token{Raw: `'head \'style\''`, Quoted: true}, want: "head 'style'"},
		{token: Token{Raw: `"selector\\d"`, Quoted: true}, want: `selector\d`},
		{token: Token{Raw: "literal"}, want: "literal"},
	}
	for _, test := range tests {
		if got := test.token.Unquoted(); got != test.want {
			t.Fatalf("Unquoted(%#v) = %q, want %q", test.token, got, test.want)
		}
	}
}

func TestTokenizerEmitsSeparatorAsDelimiter(t *testing.T) {
	grammar := ExpressionGrammar{Operators: []string{"=="}, Separator: "OR", AllowQuotedLiterals: true}
	tokens, err := Tokenizer(`${ first } OR ${ second }`, grammar)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(tokens), 3; got != want {
		t.Fatalf("token count = %d, want %d", got, want)
	}
	if got := tokens[1]; got.Kind != TokenDelimiter || got.Raw != "OR" {
		t.Fatalf("separator token = %#v, want TokenDelimiter OR", got)
	}
}

func TestTokenizerSupportsLargeTokenSequences(t *testing.T) {
	tokens, err := Tokenizer("one two three four five", ExpressionGrammar{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(tokens), 5; got != want {
		t.Fatalf("token count = %d, want %d", got, want)
	}
	if got, want := tokens[4].Raw, "five"; got != want {
		t.Fatalf("last token = %q, want %q", got, want)
	}
}

func TestTokenizerMaxTokens(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		grammar    ExpressionGrammar
		wantErr    bool
	}{
		{name: "exact limit", expression: "one two three", grammar: ExpressionGrammar{MaxTokens: 3}},
		{name: "excess token", expression: "one two three four", grammar: ExpressionGrammar{MaxTokens: 3}, wantErr: true},
		{name: "whitespace tail", expression: "one two three \t", grammar: ExpressionGrammar{MaxTokens: 3}},
		{name: "separator resets limit", expression: "one two three OR four five six", grammar: ExpressionGrammar{Separator: "OR", MaxTokens: 3}},
		{name: "separator segment excess", expression: "one two three four OR five", grammar: ExpressionGrammar{Separator: "OR", MaxTokens: 3}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Tokenizer(test.expression, test.grammar)
			if test.wantErr && err == nil {
				t.Fatal("Tokenizer() error = nil")
			}
			if !test.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTokenizeNext(t *testing.T) {
	grammar := ExpressionGrammar{Separator: "OR", MaxTokens: 3}
	remainder, tokens, err := TokenizeNext("one OR two three", grammar)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := remainder, " two three"; got != want {
		t.Fatalf("remainder = %q, want %q", got, want)
	}
	if got, want := len(tokens), 1; got != want || tokens[0].Raw != "one" {
		t.Fatalf("tokens = %#v, want [one]", tokens)
	}
	remainder, tokens, err = TokenizeNext(remainder, grammar)
	if err != nil {
		t.Fatal(err)
	}
	if remainder != "" || len(tokens) != 2 {
		t.Fatalf("final segment = (%q, %#v), want empty remainder and two tokens", remainder, tokens)
	}

	for _, expression := range []string{"", "OR one", "one OR"} {
		if _, _, err := TokenizeNext(expression, grammar); err == nil {
			t.Errorf("TokenizeNext(%q) error = nil", expression)
		}
	}
	if _, _, err := TokenizeNext("one OR OR two", grammar); err == nil {
		t.Fatal("TokenizeNext() accepted a repeated separator")
	}
}
