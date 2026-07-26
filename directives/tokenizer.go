package directives

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TokenKind identifies one lexical unit in a directive expression.
type TokenKind uint8

const (
	TokenLiteral TokenKind = iota
	TokenVariable
	TokenOperator
)

// Token preserves one expression token and its byte range in the source.
type Token struct {
	Kind   TokenKind
	Raw    string
	Quoted bool
	Start  int
	End    int
}
type Tokens []Token

// ExpressionGrammar configures tokenization for one directive expression
// syntax. Operators are exact and case-sensitive. Multi-word operators match
// their words with one or more whitespace characters between them.
type ExpressionGrammar struct {
	Operators           []string
	AllowQuotedLiterals bool
}

// CompiledExpressionGrammar is an immutable, tokenization-ready grammar.
type CompiledExpressionGrammar struct {
	operators           []grammarOperator
	allowQuotedLiterals bool
}

type grammarOperator struct {
	raw      string
	words    []string
	symbolic bool
}

func (tokens Tokens) ContainsVariable() bool {
	return slices.ContainsFunc(tokens, Token.IsVariable)
}

func (token Token) IsVariable() bool { return token.Kind == TokenVariable }
func (token Token) IsOperand() bool  { return token.Kind == TokenVariable || token.Kind == TokenLiteral }
func (token Token) IsOperator() bool { return token.Kind == TokenOperator }

// CompileExpressionGrammar validates and compiles a grammar for reuse.
func CompileExpressionGrammar(grammar ExpressionGrammar) (CompiledExpressionGrammar, error) {
	operators, err := tokenizeOperators(grammar)
	if err != nil {
		return CompiledExpressionGrammar{}, err
	}
	return CompiledExpressionGrammar{operators: operators, allowQuotedLiterals: grammar.AllowQuotedLiterals}, nil
}

// MustCompileExpressionGrammar compiles a static grammar or panics.
func MustCompileExpressionGrammar(grammar ExpressionGrammar) CompiledExpressionGrammar {
	compiled, err := CompileExpressionGrammar(grammar)
	if err != nil {
		panic(err)
	}
	return compiled
}

// Tokenizer tokenizes one directive expression according to grammar.
func Tokenizer(expression string, grammar ExpressionGrammar) ([]Token, error) {
	compiled, err := CompileExpressionGrammar(grammar)
	if err != nil {
		return nil, err
	}
	return compiled.Tokenize(expression)
}

func (grammar CompiledExpressionGrammar) Tokenize(expression string) ([]Token, error) {
	var tokens []Token
	for cursor := 0; cursor < len(expression); {
		cursor = skipExpressionWhitespace(expression, cursor)
		if cursor == len(expression) {
			break
		}
		start := cursor
		if strings.HasPrefix(expression[cursor:], "${") {
			end, err := scanVariableToken(expression, cursor)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, Token{Kind: TokenVariable, Raw: expression[start:end], Start: start, End: end})
			cursor = end
			continue
		}
		if expression[cursor] == '\'' || expression[cursor] == '"' {
			if !grammar.allowQuotedLiterals {
				return nil, fmt.Errorf("quoted literals are not allowed at byte %d", cursor)
			}
			end, err := scanQuotedToken(expression, cursor)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, Token{Kind: TokenLiteral, Raw: expression[start:end], Quoted: true, Start: start, End: end})
			cursor = end
			continue
		}
		if _, end, ok := matchOperator(expression, cursor, grammar.operators); ok {
			tokens = append(tokens, Token{Kind: TokenOperator, Raw: expression[start:end], Start: start, End: end})
			cursor = end
			continue
		}

		cursor = scanLiteralToken(expression, cursor, grammar.operators)
		tokens = append(tokens, Token{Kind: TokenLiteral, Raw: expression[start:cursor], Start: start, End: cursor})
	}
	return tokens, nil
}

func tokenizeOperators(grammar ExpressionGrammar) ([]grammarOperator, error) {
	operators := make([]grammarOperator, 0, len(grammar.Operators))
	seen := map[string]bool{}
	for _, raw := range grammar.Operators {
		words := strings.Fields(raw)
		if len(words) == 0 {
			return nil, fmt.Errorf("expression grammar contains an empty operator")
		}
		canonical := strings.Join(words, " ")
		if seen[canonical] {
			return nil, fmt.Errorf("expression grammar contains an ambiguous operator %q", raw)
		}
		seen[canonical] = true
		operators = append(operators, grammarOperator{raw: canonical, words: words, symbolic: !operatorWords(words)})
	}
	sort.SliceStable(operators, func(left, right int) bool {
		return len(operators[left].raw) > len(operators[right].raw)
	})
	return operators, nil
}

func operatorWords(words []string) bool {
	for _, word := range words {
		for _, char := range word {
			if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' {
				return false
			}
		}
	}
	return true
}

func matchOperator(expression string, start int, operators []grammarOperator) (grammarOperator, int, bool) {
	for _, operator := range operators {
		if operator.symbolic {
			if strings.HasPrefix(expression[start:], operator.raw) {
				return operator, start + len(operator.raw), true
			}
			continue
		}
		if start > 0 && !expressionWhitespaceBefore(expression, start) {
			continue
		}
		cursor := start
		matched := true
		for index, word := range operator.words {
			if !strings.HasPrefix(expression[cursor:], word) {
				matched = false
				break
			}
			cursor += len(word)
			if index < len(operator.words)-1 {
				next := skipExpressionWhitespace(expression, cursor)
				if next == cursor {
					matched = false
					break
				}
				cursor = next
			}
		}
		if matched && (cursor == len(expression) || expressionWhitespaceAt(expression, cursor)) {
			return operator, cursor, true
		}
	}
	return grammarOperator{}, start, false
}

func scanLiteralToken(expression string, start int, operators []grammarOperator) int {
	cursor := start
	for cursor < len(expression) {
		if expressionWhitespaceAt(expression, cursor) || strings.HasPrefix(expression[cursor:], "${") || expression[cursor] == '\'' || expression[cursor] == '"' {
			break
		}
		if operator, _, ok := matchOperator(expression, cursor, operators); ok && operator.symbolic {
			break
		}
		_, size := utf8.DecodeRuneInString(expression[cursor:])
		cursor += size
	}
	return cursor
}

func scanVariableToken(expression string, start int) (int, error) {
	quote := byte(0)
	escaped := false
	for cursor := start + len("${"); cursor < len(expression); cursor++ {
		char := expression[cursor]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			continue
		}
		if char == '}' {
			return cursor + 1, nil
		}
	}
	return 0, fmt.Errorf("unterminated variable at byte %d", start)
}

func scanQuotedToken(expression string, start int) (int, error) {
	quote := expression[start]
	escaped := false
	for cursor := start + 1; cursor < len(expression); cursor++ {
		char := expression[cursor]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == quote {
			return cursor + 1, nil
		}
	}
	return 0, fmt.Errorf("unterminated quoted literal at byte %d", start)
}

func skipExpressionWhitespace(expression string, start int) int {
	for start < len(expression) {
		char, size := utf8.DecodeRuneInString(expression[start:])
		if !unicode.IsSpace(char) {
			break
		}
		start += size
	}
	return start
}

func expressionWhitespaceAt(expression string, start int) bool {
	char, _ := utf8.DecodeRuneInString(expression[start:])
	return unicode.IsSpace(char)
}

func expressionWhitespaceBefore(expression string, end int) bool {
	if end <= 0 || end > len(expression) {
		return false
	}
	runeValue, _ := utf8.DecodeLastRuneInString(expression[:end])
	return unicode.IsSpace(runeValue)
}
