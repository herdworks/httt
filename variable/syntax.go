package variable

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Syntax selects the surface syntax accepted by RenderString.
type Syntax int

const htVarPrefix = "--ht-var("

const (
	// SyntaxDefault recognizes ${ ... } variables.
	SyntaxDefault Syntax = iota

	// SyntaxCSS recognizes --ht-var("...") and --ht-var('...') calls.
	// The quoted argument follows the default expression grammar. An optional
	// second argument is returned only when the expression resolves to nil.
	SyntaxCSS
)

var errInvalidHTVarCall = errors.New("invalid --ht-var call")

func atWordBoundary(source string, start int) bool {
	if start == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(source[:start])
	if r == utf8.RuneError {
		return true
	}
	return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
}

func readHTVarCall(source string, start int) (string, string, bool, int, error) {
	cursor := skipCSSWhitespace(source, start+len(htVarPrefix))
	if cursor >= len(source) {
		return "", "", false, 0, htVarError(source, start, "expected quoted argument, found end of input")
	}

	quote := source[cursor]
	if quote != '"' && quote != '\'' {
		return "", "", false, 0, htVarError(source, start, fmt.Sprintf("expected quoted argument, found %q", string(source[cursor])))
	}
	body, closeQuote, err := readQuotedBody(source, cursor, quote)
	if err != nil {
		return "", "", false, 0, htVarError(source, start, err.Error())
	}
	if strings.TrimSpace(body) == "" {
		return "", "", false, 0, htVarError(source, start, "empty variable expression")
	}

	cursor = skipCSSWhitespace(source, closeQuote+1)
	if cursor >= len(source) {
		return "", "", false, 0, htVarError(source, start, "missing closing )")
	}
	switch source[cursor] {
	case ')':
		return body, "", false, cursor + 1, nil
	case ',':
		cursor++
	default:
		return "", "", false, 0, htVarError(source, start, fmt.Sprintf("expected , or ), found %q", string(source[cursor])))
	}

	fallback, end, err := readFallback(source, cursor)
	if err != nil {
		return "", "", false, 0, htVarError(source, start, err.Error())
	}
	return body, fallback, true, end, nil
}

func readFallback(source string, cursor int) (string, int, error) {
	cursor = skipCSSWhitespace(source, cursor)
	if cursor >= len(source) {
		return "", 0, errors.New("missing fallback value")
	}

	if quote := source[cursor]; quote == '"' || quote == '\'' {
		body, closeQuote, err := readQuotedBody(source, cursor, quote)
		if err != nil {
			return "", 0, err
		}
		cursor = skipCSSWhitespace(source, closeQuote+1)
		if cursor >= len(source) {
			return "", 0, errors.New("missing closing )")
		}
		if source[cursor] != ')' {
			return "", 0, fmt.Errorf("expected ), found %q", string(source[cursor]))
		}
		return body, cursor + 1, nil
	}

	start := cursor
	depth := 0
	for cursor < len(source) {
		char := source[cursor]
		if char == '"' || char == '\'' {
			_, closeQuote, err := readQuotedBody(source, cursor, char)
			if err != nil {
				return "", 0, err
			}
			cursor = closeQuote + 1
			continue
		}
		switch char {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				raw := strings.TrimSpace(source[start:cursor])
				if raw == "" {
					return "", 0, errors.New("empty fallback value")
				}
				return raw, cursor + 1, nil
			}
			depth--
		}
		cursor++
	}
	return "", 0, errors.New("missing closing )")
}

func readQuotedBody(source string, openQuote int, quote byte) (string, int, error) {
	var builder strings.Builder
	for cursor := openQuote + 1; cursor < len(source); cursor++ {
		char := source[cursor]
		if char == '\\' {
			if cursor+1 >= len(source) {
				return "", 0, errors.New("unterminated string")
			}
			next := source[cursor+1]
			switch next {
			case '\\', '"', '\'':
				builder.WriteByte(next)
			default:
				builder.WriteByte(char)
				builder.WriteByte(next)
			}
			cursor++
			continue
		}
		if char == quote {
			return builder.String(), cursor, nil
		}
		builder.WriteByte(char)
	}
	return "", 0, errors.New("unterminated string")
}

func skipCSSWhitespace(source string, cursor int) int {
	for cursor < len(source) {
		char := source[cursor]
		if char != ' ' && char != '\t' && char != '\r' && char != '\n' {
			break
		}
		cursor++
	}
	return cursor
}

func htVarError(source string, start int, detail string) error {
	excerpt := source[start:]
	if len(excerpt) > 40 {
		excerpt = excerpt[:40] + "..."
	}
	return fmt.Errorf("%w: %w: %s: %q", errInvalidVariable, errInvalidHTVarCall, detail, excerpt)
}
