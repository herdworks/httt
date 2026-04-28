package value

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	texttemplate "text/template"
	"unicode"
	"unicode/utf8"
)

const templateName = "ht-value"

// TYPES =======================================================================

// tmpl wraps a normalized template value source string.
type tmpl struct {
	source     string
	normalized string
	syntax     Syntax
	funcs      FuncMap
}

// Getter resolves HyperTemplates-style keypaths.
type Getter interface {
	Get(string) any
}

// FuncMap registers user-defined template functions. It is an alias of
// text/template.FuncMap so callers can pass ordinary template functions
// directly.
type FuncMap = texttemplate.FuncMap

// METHODS =====================================================================

// execute renders a parsed template value. If the rendered output contains
// text/template's "<no value>" marker (indicating that at least one action
// resolved to nil), the original user-visible template source is emitted
// instead. This all-or-nothing behavior keeps unresolved templates visible in
// the output rather than leaking "<no value>" placeholders; callers that need
// partial resolution on multi-action templates should annotate individual
// actions with "| default \"...\"".
func (t *tmpl) execute(writer io.Writer, data Getter) error {
	compiled, err := buildTemplate(t.normalized, data, t.funcs)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	if err = compiled.Execute(&buffer, data); err != nil {
		return fmt.Errorf("failed executing template value %q: %w", templateName, err)
	}

	result := buffer.String()
	if strings.Contains(result, "<no value>") {
		result = t.source
	}

	if _, err = io.WriteString(writer, result); err != nil {
		return err
	}

	return nil
}

// executeString renders a parsed template value into a string.
func (t *tmpl) executeString(data Getter) (string, error) {
	var buffer bytes.Buffer

	if err := t.execute(&buffer, data); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

// FUNCTIONS ===================================================================

// Render parses and executes template values in one step.
func Render(source string, data Getter, syntax Syntax, funcs ...FuncMap) (string, error) {
	parsed, err := parseWithFuncs(source, syntax, mergeFuncMaps(funcs...))
	if err != nil {
		return "", err
	}

	return parsed.executeString(data)
}

// parse validates and normalizes a template value source.
func parse(source string, syntax Syntax) (*tmpl, error) {
	return parseWithFuncs(source, syntax, nil)
}

func parseWithFuncs(source string, syntax Syntax, funcs FuncMap) (*tmpl, error) {
	normalized, err := rewriteWithFuncs(source, syntax, funcs)
	if err != nil {
		return nil, err
	}

	if _, err = buildTemplate(normalized, nil, funcs); err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidValue, err)
	}

	return &tmpl{
		source:     source,
		normalized: normalized,
		syntax:     syntax,
		funcs:      funcs,
	}, nil
}

// validate checks whether a template value source can be normalized and parsed.
func validate(source string, syntax Syntax) error {
	return validateWithFuncs(source, syntax, nil)
}

func validateWithFuncs(source string, syntax Syntax, funcs FuncMap) error {
	_, err := parseWithFuncs(source, syntax, funcs)
	return err
}

// rewrite normalizes a template source to default-syntax form and rewrites
// bare keypath actions into explicit httd function calls. For SyntaxCSS, a
// pre-pass translates htfmt(...) calls to ${ ... } actions before the main
// rewrite loop runs.
func rewrite(source string, syntax Syntax) (string, error) {
	return rewriteWithFuncs(source, syntax, nil)
}

func rewriteWithFuncs(source string, syntax Syntax, funcs FuncMap) (string, error) {
	if syntax == SyntaxCSS {
		translated, err := rewriteCSS(source)
		if err != nil {
			return "", err
		}
		source = translated
	}

	var builder strings.Builder
	cursor := 0

	for cursor < len(source) {
		offset := strings.Index(source[cursor:], "${")
		if offset < 0 {
			builder.WriteString(source[cursor:])
			break
		}

		start := cursor + offset
		builder.WriteString(source[cursor:start])

		action, err := readAction(source, start)
		if err != nil {
			return "", err
		}

		body, err := rewriteAction(action.body, funcs)
		if err != nil {
			return "", err
		}

		builder.WriteString("${")
		if action.leftTrim {
			builder.WriteString("-")
		}
		builder.WriteString(action.leadingWhitespace)
		builder.WriteString(body)
		builder.WriteString(action.trailingWhitespace)
		if action.rightTrim {
			builder.WriteString("-")
		}
		builder.WriteString("}")

		cursor = action.end
	}

	return builder.String(), nil
}

func buildTemplate(source string, data Getter, funcs FuncMap) (parsed *texttemplate.Template, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			parsed = nil
			err = fmt.Errorf("failed configuring template value %q: %v", templateName, recovered)
		}
	}()

	compiledFuncs := texttemplate.FuncMap{
		"httd": func(keypaths string) any {
			return httdFunc(data, keypaths)
		},
		"default": defaultFunc,
	}
	for name, fn := range funcs {
		if _, reserved := compiledFuncs[name]; reserved {
			return nil, fmt.Errorf("template value function %q is reserved", name)
		}
		compiledFuncs[name] = fn
	}

	compiled := texttemplate.New(templateName).Delims("${", "}").Funcs(compiledFuncs)

	parsed, err = compiled.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("failed parsing template value %q: %w", templateName, err)
	}

	return parsed, nil
}

func mergeFuncMaps(funcMaps ...FuncMap) FuncMap {
	var merged FuncMap
	for _, funcs := range funcMaps {
		if len(funcs) == 0 {
			continue
		}
		if merged == nil {
			merged = FuncMap{}
		}
		for name, fn := range funcs {
			merged[name] = fn
		}
	}
	return merged
}

// defaultFunc returns defaultValue when value is nil or an empty string, and
// returns value otherwise. It is registered in the template func map as
// "default" and is intended as a pipeline terminator for cases where a
// template body may resolve to nothing — particularly the CSS-syntax
// --ht-value("<body>", <fallback>) form.
func defaultFunc(defaultValue string, value any) any {
	if value == nil {
		return defaultValue
	}
	if s, ok := value.(string); ok && s == "" {
		return defaultValue
	}
	return value
}

// httdFunc is registered in the template func map as "httd" and resolves
// comma-separated keypaths against the Getter, returning the first non-nil
// value (or nil if all keypaths miss).
func httdFunc(data Getter, keypaths string) any {
	if data == nil {
		return nil
	}

	for _, keypath := range strings.Split(keypaths, ",") {
		keypath = strings.TrimSpace(keypath)
		if keypath == "" {
			continue
		}

		value := data.Get(keypath)
		if value != nil {
			return value
		}
	}

	return nil
}

type action struct {
	body               string
	end                int
	leftTrim           bool
	rightTrim          bool
	leadingWhitespace  string
	trailingWhitespace string
}

func readAction(source string, start int) (*action, error) {
	cursor := start + len("${")
	leftTrim := false

	if cursor < len(source) && source[cursor] == '-' {
		leftTrim = true
		cursor += 1
	}

	bodyStart := cursor
	var quote byte
	escaped := false
	inComment := false

	for cursor < len(source) {
		char := source[cursor]

		if inComment {
			if char == '*' && cursor+1 < len(source) && source[cursor+1] == '/' {
				inComment = false
				cursor += 2
				continue
			}

			cursor += 1
			continue
		}

		switch quote {
		case '`':
			if char == '`' {
				quote = 0
			}
			cursor += 1
			continue
		case '"', '\'':
			if escaped {
				escaped = false
				cursor += 1
				continue
			}
			if char == '\\' {
				escaped = true
				cursor += 1
				continue
			}
			if char == quote {
				quote = 0
			}
			cursor += 1
			continue
		}

		if char == '/' && cursor+1 < len(source) && source[cursor+1] == '*' {
			inComment = true
			cursor += 2
			continue
		}

		switch char {
		case '"', '\'', '`':
			quote = char
		case '}':
			rawBody := source[bodyStart:cursor]
			rightTrim := false
			if strings.HasSuffix(rawBody, "-") {
				rightTrim = true
				rawBody = strings.TrimSuffix(rawBody, "-")
			}

			leadingWhitespace, trimmedBody, trailingWhitespace := splitWhitespace(rawBody)

			return &action{
				body:               trimmedBody,
				end:                cursor + 1,
				leftTrim:           leftTrim,
				rightTrim:          rightTrim,
				leadingWhitespace:  leadingWhitespace,
				trailingWhitespace: trailingWhitespace,
			}, nil
		}

		cursor += 1
	}

	return nil, fmt.Errorf("%w: missing closing } in %q", errUnterminatedAction, source[start:])
}

func splitWhitespace(source string) (string, string, string) {
	left := 0
	for left < len(source) {
		r, size := utf8.DecodeRuneInString(source[left:])
		if !unicode.IsSpace(r) {
			break
		}
		left += size
	}

	right := len(source)
	for right > left {
		r, size := utf8.DecodeLastRuneInString(source[:right])
		if !unicode.IsSpace(r) {
			break
		}
		right -= size
	}

	return source[:left], source[left:right], source[right:]
}

func rewriteAction(source string, funcs FuncMap) (string, error) {
	body := strings.TrimSpace(source)
	if body == "" {
		return "", nil
	}

	if strings.HasPrefix(body, "/*") {
		return "", fmt.Errorf("%w: %s: %q", errInvalidValue, errTemplateComment, body)
	}

	body, err := normalizeSingleQuotedStrings(body)
	if err != nil {
		return "", err
	}

	head, tail := splitPipeline(body)
	head = strings.TrimSpace(head)
	if head == "" {
		return "", fmt.Errorf("%w: empty action", errInvalidValue)
	}

	// Explicit function call: pass the body through verbatim and let
	// text/template validate the tail against its FuncMap.
	if reservedFunction(firstToken(head), funcs) {
		return body, nil
	}

	for _, segment := range strings.Split(head, ",") {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			return "", fmt.Errorf("%w: %w: empty keypath in %q", errInvalidValue, errInvalidKeypath, head)
		}
		if strings.ContainsFunc(trimmed, unicode.IsSpace) {
			return "", fmt.Errorf("%w: %w: %q", errInvalidValue, errInvalidKeypath, trimmed)
		}
	}

	rewritten := fmt.Sprintf("httd %s", strconv.Quote(head))
	if tail == "" {
		return rewritten, nil
	}

	return rewritten + " " + strings.TrimLeftFunc(tail, unicode.IsSpace), nil
}

func normalizeSingleQuotedStrings(source string) (string, error) {
	var builder strings.Builder
	cursor := 0

	for cursor < len(source) {
		char := source[cursor]

		switch char {
		case '\'':
			value, closeQuote, err := readQuotedBody(source, cursor, '\'')
			if err != nil {
				return "", fmt.Errorf("%w: %v", errInvalidValue, err)
			}
			builder.WriteString(strconv.Quote(value))
			cursor = closeQuote + 1
			continue
		case '"', '`':
			end, err := skipQuotedString(source, cursor, char)
			if err != nil {
				return "", fmt.Errorf("%w: %v", errInvalidValue, err)
			}
			builder.WriteString(source[cursor:end])
			cursor = end
			continue
		case '/':
			if cursor+1 < len(source) && source[cursor+1] == '*' {
				end := strings.Index(source[cursor+2:], "*/")
				if end < 0 {
					return "", fmt.Errorf("%w: unterminated comment", errInvalidValue)
				}
				end = cursor + 2 + end + 2
				builder.WriteString(source[cursor:end])
				cursor = end
				continue
			}
		}

		builder.WriteByte(char)
		cursor += 1
	}

	return builder.String(), nil
}

func skipQuotedString(source string, openQuote int, quote byte) (int, error) {
	cursor := openQuote + 1

	for cursor < len(source) {
		char := source[cursor]

		if quote == '`' {
			if char == '`' {
				return cursor + 1, nil
			}
			cursor += 1
			continue
		}

		if char == '\\' {
			cursor += 2
			continue
		}
		if char == quote {
			return cursor + 1, nil
		}
		cursor += 1
	}

	return 0, errors.New("unterminated string")
}

func splitPipeline(source string) (string, string) {
	var quote byte
	escaped := false
	inComment := false
	depth := 0

	for index := 0; index < len(source); index += 1 {
		char := source[index]

		if inComment {
			if char == '*' && index+1 < len(source) && source[index+1] == '/' {
				inComment = false
				index += 1
			}
			continue
		}

		switch quote {
		case '`':
			if char == '`' {
				quote = 0
			}
			continue
		case '"', '\'':
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

		if char == '/' && index+1 < len(source) && source[index+1] == '*' {
			inComment = true
			index += 1
			continue
		}

		switch char {
		case '"', '\'', '`':
			quote = char
		case '(':
			depth += 1
		case ')':
			if depth > 0 {
				depth -= 1
			}
		case '|':
			if depth == 0 {
				return source[:index], source[index:]
			}
		}
	}

	return source, ""
}

func firstToken(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}

	for index, char := range source {
		if unicode.IsSpace(char) || char == '|' {
			return source[:index]
		}
	}

	return source
}

func reservedFunction(name string, funcs FuncMap) bool {
	switch name {
	case "httd", "default":
		return true
	}
	_, ok := funcs[name]
	return ok
}
