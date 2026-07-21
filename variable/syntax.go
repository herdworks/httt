package variable

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Syntax selects the surface syntax the parser accepts for template actions.
type Syntax int

const (
	// SyntaxDefault recognizes ${ ... } actions.
	SyntaxDefault Syntax = iota

	// SyntaxCSS recognizes --ht-value("...") and --ht-value('...') calls.
	// The quoted argument is the action body and follows the same grammar
	// as a default-syntax action. An optional second argument is applied as a
	// CSS fallback when the action renders no value.
	SyntaxCSS
)

// errInvalidHTValueCall indicates a --ht-value(...) call is malformed.
var errInvalidHTValueCall = errors.New("invalid --ht-value call")

const htValuePrefix = "--ht-value("

// rewriteCSS replaces --ht-value("<body>") and --ht-value('<body>') calls with
// ${ <body> } actions. An optional second argument is treated as a CSS fallback
// after the expression pipeline. Text outside of --ht-value(...) calls passes
// through unchanged.
func rewriteCSS(source string) (string, error) {
	var builder strings.Builder
	cursor := 0

	for cursor < len(source) {
		offset := strings.Index(source[cursor:], htValuePrefix)
		if offset < 0 {
			builder.WriteString(source[cursor:])
			break
		}

		start := cursor + offset

		if !atWordBoundary(source, start) {
			end := start + len(htValuePrefix)
			builder.WriteString(source[cursor:end])
			cursor = end
			continue
		}

		builder.WriteString(source[cursor:start])

		body, fallback, hasFallback, end, err := readHTValueCall(source, start)
		if err != nil {
			return "", err
		}
		if hasFallback {
			body = appendCSSDefault(body, fallback)
		}

		builder.WriteString("${ ")
		builder.WriteString(body)
		builder.WriteString(" }")

		cursor = end
	}

	return builder.String(), nil
}

// atWordBoundary reports whether the rune preceding start is a non-identifier
// rune (or start is at position 0). Identifier runes are letters, digits,
// underscore, and hyphen (the latter so that "---ht-value(" does not match
// the embedded "--ht-value(").
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

// readHTValueCall parses a --ht-value(...) call starting at start, which
// points at the first '-' of "--ht-value(". It returns the decoded body, the
// decoded fallback (if present), whether a fallback was present, the byte index
// just past the closing ')', and an error.
func readHTValueCall(source string, start int) (string, string, bool, int, error) {
	cursor := start + len(htValuePrefix)
	cursor = skipCSSWhitespace(source, cursor)

	if cursor >= len(source) {
		return "", "", false, 0, htValueError(source, start, "expected quoted argument, found end of input")
	}

	quote := source[cursor]
	if quote != '"' && quote != '\'' {
		return "", "", false, 0, htValueError(source, start, fmt.Sprintf("expected quoted argument, found %q", string(source[cursor])))
	}

	body, closeQuote, err := readQuotedBody(source, cursor, quote)
	if err != nil {
		return "", "", false, 0, htValueError(source, start, err.Error())
	}

	if strings.TrimSpace(body) == "" {
		return "", "", false, 0, htValueError(source, start, "empty template argument")
	}

	cursor = closeQuote + 1
	cursor = skipCSSWhitespace(source, cursor)

	if cursor >= len(source) {
		return "", "", false, 0, htValueError(source, start, "missing closing )")
	}

	switch source[cursor] {
	case ')':
		return body, "", false, cursor + 1, nil
	case ',':
		cursor += 1
	default:
		return "", "", false, 0, htValueError(source, start, fmt.Sprintf("expected , or ), found %q", string(source[cursor])))
	}

	fallback, end, err := readFallback(source, cursor)
	if err != nil {
		return "", "", false, 0, htValueError(source, start, err.Error())
	}

	return body, fallback, true, end, nil
}

func appendCSSDefault(body string, fallback string) string {
	body = strings.TrimRightFunc(body, unicode.IsSpace)
	return body + " | " + cssDefaultFuncName + " " + strconv.Quote(fallback)
}

// readFallback reads the second argument of a --ht-value(...) call starting
// just after the comma. A quoted argument is decoded the same way as the first
// argument. An unquoted argument is read verbatim up to the balancing ')',
// respecting nested parens and quoted strings. Returns the fallback text, the
// byte index just past the closing ')', and an error.
func readFallback(source string, cursor int) (string, int, error) {
	cursor = skipCSSWhitespace(source, cursor)
	if cursor >= len(source) {
		return "", 0, errors.New("missing fallback value")
	}

	if c := source[cursor]; c == '"' || c == '\'' {
		body, closeQuote, err := readQuotedBody(source, cursor, c)
		if err != nil {
			return "", 0, err
		}
		cursor = closeQuote + 1
		cursor = skipCSSWhitespace(source, cursor)
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
		c := source[cursor]
		if c == '"' || c == '\'' {
			_, closeQuote, err := readQuotedBody(source, cursor, c)
			if err != nil {
				return "", 0, err
			}
			cursor = closeQuote + 1
			continue
		}
		if c == '(' {
			depth += 1
			cursor += 1
			continue
		}
		if c == ')' {
			if depth == 0 {
				raw := strings.TrimRight(source[start:cursor], " \t\r\n")
				if raw == "" {
					return "", 0, errors.New("empty fallback value")
				}
				return raw, cursor + 1, nil
			}
			depth -= 1
			cursor += 1
			continue
		}
		cursor += 1
	}

	return "", 0, errors.New("missing closing )")
}

func skipCSSWhitespace(source string, cursor int) int {
	for cursor < len(source) {
		c := source[cursor]
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			return cursor
		}
		cursor += 1
	}
	return cursor
}

// readQuotedBody reads a quoted string starting at openQuote (the opening quote
// byte). It returns the decoded body, the byte index of the closing quote, and
// an error. Backslash escapes handled: \\ -> \, \" -> ", \' -> '. Other \X
// sequences pass through as \X.
func readQuotedBody(source string, openQuote int, quote byte) (string, int, error) {
	var builder strings.Builder
	cursor := openQuote + 1

	for cursor < len(source) {
		c := source[cursor]
		if c == '\\' {
			if cursor+1 >= len(source) {
				return "", 0, errors.New("unterminated string")
			}
			next := source[cursor+1]
			switch next {
			case '\\', '"', '\'':
				builder.WriteByte(next)
			default:
				builder.WriteByte(c)
				builder.WriteByte(next)
			}
			cursor += 2
			continue
		}
		if c == quote {
			return builder.String(), cursor, nil
		}
		builder.WriteByte(c)
		cursor += 1
	}

	return "", 0, errors.New("unterminated string")
}

func htValueError(source string, start int, detail string) error {
	excerpt := source[start:]
	if len(excerpt) > 40 {
		excerpt = excerpt[:40] + "..."
	}
	return fmt.Errorf("%w: %w: %s: %q", errInvalidValue, errInvalidHTValueCall, detail, excerpt)
}
