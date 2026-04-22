package format

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ErrInvalidHTFormatCall indicates a --ht-format(...) call is malformed.
var ErrInvalidHTFormatCall = errors.New("invalid --ht-format call")

const htFormatPrefix = "--ht-format("

// rewriteCSS replaces --ht-format("<body>") and --ht-format('<body>') calls with
// ${ <body> } actions. An optional second argument provides a fallback value
// used when the template body evaluates to nil or an empty string; when
// present, the emission becomes ${ <body> | default "<fallback>" }.
// Text outside of --ht-format(...) calls passes through unchanged.
func rewriteCSS(source string) (string, error) {
	var builder strings.Builder
	cursor := 0

	for cursor < len(source) {
		offset := strings.Index(source[cursor:], htFormatPrefix)
		if offset < 0 {
			builder.WriteString(source[cursor:])
			break
		}

		start := cursor + offset

		if !atWordBoundary(source, start) {
			end := start + len(htFormatPrefix)
			builder.WriteString(source[cursor:end])
			cursor = end
			continue
		}

		builder.WriteString(source[cursor:start])

		body, fallback, hasFallback, end, err := readHTFormatCall(source, start)
		if err != nil {
			return "", err
		}

		builder.WriteString("${ ")
		builder.WriteString(body)
		if hasFallback {
			builder.WriteString(" | default ")
			builder.WriteString(strconv.Quote(fallback))
		}
		builder.WriteString(" }")

		cursor = end
	}

	return builder.String(), nil
}

// atWordBoundary reports whether the rune preceding start is a non-identifier
// rune (or start is at position 0). Identifier runes are letters, digits,
// underscore, and hyphen (the latter so that "---ht-format(" does not match
// the embedded "--ht-format(").
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

// readHTFormatCall parses a --ht-format(...) call starting at start, which
// points at the first '-' of "--ht-format(". It returns the decoded body, the
// decoded fallback (empty if absent), whether a fallback was present, the byte
// index just past the closing ')', and an error.
func readHTFormatCall(source string, start int) (string, string, bool, int, error) {
	cursor := start + len(htFormatPrefix)
	cursor = skipCSSWhitespace(source, cursor)

	if cursor >= len(source) {
		return "", "", false, 0, htFormatError(source, start, "expected quoted argument, found end of input")
	}

	quote := source[cursor]
	if quote != '"' && quote != '\'' {
		return "", "", false, 0, htFormatError(source, start, fmt.Sprintf("expected quoted argument, found %q", string(source[cursor])))
	}

	body, closeQuote, err := readQuotedBody(source, cursor, quote)
	if err != nil {
		return "", "", false, 0, htFormatError(source, start, err.Error())
	}

	if strings.TrimSpace(body) == "" {
		return "", "", false, 0, htFormatError(source, start, "empty template argument")
	}

	cursor = closeQuote + 1
	cursor = skipCSSWhitespace(source, cursor)

	if cursor >= len(source) {
		return "", "", false, 0, htFormatError(source, start, "missing closing )")
	}

	switch source[cursor] {
	case ')':
		return body, "", false, cursor + 1, nil
	case ',':
		cursor += 1
	default:
		return "", "", false, 0, htFormatError(source, start, fmt.Sprintf("expected , or ), found %q", string(source[cursor])))
	}

	fallback, end, err := readFallback(source, cursor)
	if err != nil {
		return "", "", false, 0, htFormatError(source, start, err.Error())
	}

	return body, fallback, true, end, nil
}

// readFallback reads the second argument of a --ht-format(...) call starting
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

func htFormatError(source string, start int, detail string) error {
	excerpt := source[start:]
	if len(excerpt) > 40 {
		excerpt = excerpt[:40] + "..."
	}
	return fmt.Errorf("%w: %w: %s: %q", ErrInvalidTemplate, ErrInvalidHTFormatCall, detail, excerpt)
}
