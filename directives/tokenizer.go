package directives

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
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

// TokenKind identifies one lexical unit in a directive expression.
type TokenKind uint8

const (
	TokenLiteral TokenKind = iota
	TokenVariable
	TokenOperator
	TokenDelimiter
)

// ExpressionGrammar configures tokenization for one directive expression
// syntax. Operators are exact and case-sensitive. Multi-word operators match
// their words with one or more whitespace characters between them.
type ExpressionGrammar struct {
	Operators []string
	// Separator is emitted as a TokenDelimiter. Like a word operator, it is
	// case-sensitive and whitespace-delimited.
	Separator           string
	AllowQuotedLiterals bool
	// MaxTokens limits tokens in one expression segment. When Separator is
	// configured, each delimiter-bounded segment has its own limit. Zero
	// permits any number of tokens.
	MaxTokens int
}

func (tokens Tokens) ContainsVariable() bool {
	return slices.ContainsFunc(tokens, Token.IsVariable)
}

// Expression reconstructs a normalized expression from its lexical tokens.
// Tokenization intentionally discards inter-token whitespace, so tokens are
// joined with one space.
func (tokens Tokens) Expression() string {
	raw := make([]string, len(tokens))
	for index, token := range tokens {
		raw[index] = token.Raw
	}
	return strings.Join(raw, " ")
}

func (token Token) IsVariable() bool  { return token.Kind == TokenVariable }
func (token Token) IsOperand() bool   { return token.Kind == TokenVariable || token.Kind == TokenLiteral }
func (token Token) IsOperator() bool  { return token.Kind == TokenOperator }
func (token Token) IsDelimiter() bool { return token.Kind == TokenDelimiter }

// Unquoted returns a quoted literal token's contents with quote and
// backslash escapes removed. Non-quoted tokens retain their raw text.
func (token Token) Unquoted() string {
	if !token.Quoted {
		return token.Raw
	}
	var value strings.Builder
	value.Grow(len(token.Raw) - 2)
	for index := 1; index < len(token.Raw)-1; index++ {
		if token.Raw[index] == '\\' {
			index++
			if token.Raw[index] != token.Raw[0] && token.Raw[index] != '\\' {
				value.WriteByte('\\')
			}
		}
		value.WriteByte(token.Raw[index])
	}
	return value.String()
}

// Tokenizer tokenizes one directive expression according to grammar.
func Tokenizer(expression string, grammar ExpressionGrammar) ([]Token, error) {
	if err := validateExpressionGrammar(grammar); err != nil {
		return nil, err
	}
	return tokenizeExpression(expression, grammar)
}

func tokenizeExpression(expression string, grammar ExpressionGrammar) ([]Token, error) {
	if grammar.MaxTokens > 0 && grammar.Separator == "" {
		return tokenizeBoundedExpression(expression, grammar)
	}

	count := 0
	segmentCount := 0
	for cursor := 0; cursor < len(expression); {
		cursor = skipExpressionWhitespace(expression, cursor)
		if cursor == len(expression) {
			break
		}
		token, end, err := tokenAt(expression, cursor, grammar)
		if err != nil {
			return nil, err
		}
		count++
		if token.IsDelimiter() {
			segmentCount = 0
		} else {
			segmentCount++
			if grammar.MaxTokens > 0 && segmentCount > grammar.MaxTokens {
				return nil, fmt.Errorf("expression segment exceeds maximum of %d tokens", grammar.MaxTokens)
			}
		}
		cursor = end
	}
	tokens := make([]Token, 0, count)
	for cursor := 0; cursor < len(expression); {
		cursor = skipExpressionWhitespace(expression, cursor)
		if cursor == len(expression) {
			break
		}
		token, end, err := tokenAt(expression, cursor, grammar)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
		cursor = end
	}
	return tokens, nil
}

func tokenizeBoundedExpression(expression string, grammar ExpressionGrammar) ([]Token, error) {
	cursor := skipExpressionWhitespace(expression, 0)
	if cursor == len(expression) {
		return make([]Token, 0), nil
	}
	tokens := make([]Token, 0, grammar.MaxTokens)
	for cursor < len(expression) {
		token, end, err := tokenAt(expression, cursor, grammar)
		if err != nil {
			return nil, err
		}
		if grammar.MaxTokens > 0 && len(tokens) == grammar.MaxTokens {
			return nil, fmt.Errorf("expression exceeds maximum of %d tokens", grammar.MaxTokens)
		}
		tokens = append(tokens, token)
		cursor = skipExpressionWhitespace(expression, end)
	}
	return tokens, nil
}

func tokenAt(expression string, start int, grammar ExpressionGrammar) (Token, int, error) {
	if strings.HasPrefix(expression[start:], "${") {
		end, err := scanVariableToken(expression, start)
		if err != nil {
			return Token{}, 0, err
		}
		return Token{Kind: TokenVariable, Raw: expression[start:end], Start: start, End: end}, end, nil
	}
	if expression[start] == '\'' || expression[start] == '"' {
		if !grammar.AllowQuotedLiterals {
			return Token{}, 0, fmt.Errorf("quoted literals are not allowed at byte %d", start)
		}
		end, err := scanQuotedToken(expression, start)
		if err != nil {
			return Token{}, 0, err
		}
		return Token{Kind: TokenLiteral, Raw: expression[start:end], Quoted: true, Start: start, End: end}, end, nil
	}
	if grammar.Separator != "" {
		if end, ok := matchExpressionOperator(expression, start, grammar.Separator); ok {
			return Token{Kind: TokenDelimiter, Raw: expression[start:end], Start: start, End: end}, end, nil
		}
	}
	if end, _, ok := matchExpressionOperators(expression, start, grammar.Operators); ok {
		return Token{Kind: TokenOperator, Raw: expression[start:end], Start: start, End: end}, end, nil
	}
	end := scanLiteralToken(expression, start, grammar.Operators)
	return Token{Kind: TokenLiteral, Raw: expression[start:end], Start: start, End: end}, end, nil
}

func validateExpressionGrammar(grammar ExpressionGrammar) error {
	if grammar.MaxTokens < 0 {
		return fmt.Errorf("expression grammar maximum tokens cannot be negative")
	}
	seen := map[string]bool{}
	for _, raw := range grammar.Operators {
		words := strings.Fields(raw)
		if len(words) == 0 {
			return fmt.Errorf("expression grammar contains an empty operator")
		}
		canonical := strings.Join(words, " ")
		if seen[canonical] {
			return fmt.Errorf("expression grammar contains an ambiguous operator %q", raw)
		}
		seen[canonical] = true
	}
	if grammar.Separator != "" && !expressionWordOperator(grammar.Separator) {
		return fmt.Errorf("expression grammar separator %q must be a word operator", grammar.Separator)
	}
	return nil
}

func expressionWordOperator(operator string) bool {
	for _, char := range operator {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' && !unicode.IsSpace(char) {
			return false
		}
	}
	return true
}

func matchExpressionOperators(expression string, start int, operators []string) (int, bool, bool) {
	bestEnd := start
	bestSymbolic := false
	for _, operator := range operators {
		if len(operator) == 0 || operator[0] != expression[start] {
			continue
		}
		if end, ok := matchExpressionOperator(expression, start, operator); ok && end > bestEnd {
			bestEnd = end
			bestSymbolic = expressionSymbolicStart(operator, 0)
		}
	}
	return bestEnd, bestSymbolic, bestEnd > start
}

func matchExpressionOperator(expression string, start int, operator string) (int, bool) {
	if expressionSymbolicStart(operator, 0) {
		if strings.HasPrefix(expression[start:], operator) {
			return start + len(operator), true
		}
		return start, false
	}
	if start > 0 && !expressionWhitespaceBefore(expression, start) {
		return start, false
	}
	cursor := start
	for index := 0; index < len(operator); {
		wordStart := index
		for index < len(operator) && !expressionWhitespaceAt(operator, index) {
			_, size := utf8.DecodeRuneInString(operator[index:])
			index += size
		}
		word := operator[wordStart:index]
		if word == "" || !strings.HasPrefix(expression[cursor:], word) {
			return start, false
		}
		cursor += len(word)
		if index < len(operator) {
			for index < len(operator) && expressionWhitespaceAt(operator, index) {
				_, size := utf8.DecodeRuneInString(operator[index:])
				index += size
			}
			next := skipExpressionWhitespace(expression, cursor)
			if next == cursor {
				return start, false
			}
			cursor = next
		}
	}
	return cursor, cursor == len(expression) || expressionWhitespaceAt(expression, cursor)
}

func scanLiteralToken(expression string, start int, operators []string) int {
	cursor := start
	for cursor < len(expression) {
		if expressionWhitespaceAt(expression, cursor) || strings.HasPrefix(expression[cursor:], "${") || expression[cursor] == '\'' || expression[cursor] == '"' {
			break
		}
		if expressionSymbolicStart(expression, cursor) {
			if _, symbolic, ok := matchExpressionOperators(expression, cursor, operators); ok && symbolic {
				break
			}
		}
		_, size := utf8.DecodeRuneInString(expression[cursor:])
		cursor += size
	}
	return cursor
}

func expressionSymbolicStart(expression string, start int) bool {
	char, _ := utf8.DecodeRuneInString(expression[start:])
	return !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_'
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
