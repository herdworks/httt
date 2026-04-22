package format

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

// Getter resolves HyperTemplates-style keypaths.
type Getter interface {
	Get(string) any
}

var (
	ErrInvalidTemplate    = errors.New("invalid format template")
	ErrTemplateComment    = errors.New("template comments are not supported")
	ErrReservedAction     = errors.New("template action is not supported")
	ErrUnterminatedAction = errors.New("unterminated template action")
)

var reservedActionKeywords = map[string]struct{}{
	"if":       {},
	"range":    {},
	"break":    {},
	"continue": {},
	"template": {},
	"block":    {},
	"with":     {},
}

var builtinFuncNames = map[string]struct{}{
	"and":      {},
	"call":     {},
	"eq":       {},
	"ge":       {},
	"gt":       {},
	"html":     {},
	"index":    {},
	"js":       {},
	"le":       {},
	"len":      {},
	"lt":       {},
	"ne":       {},
	"not":      {},
	"or":       {},
	"print":    {},
	"printf":   {},
	"println":  {},
	"slice":    {},
	"urlquery": {},
}

// Template wraps a normalized format template source string.
type Template struct {
	name       string
	source     string
	normalized string
	funcs      texttemplate.FuncMap
}

// Parse validates and normalizes a format template.
func Parse(name string, source string, funcs texttemplate.FuncMap) (*Template, error) {
	normalized, err := Rewrite(source, funcs)
	if err != nil {
		return nil, err
	}

	if _, err = buildTemplate(name, normalized, funcs, nil); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidTemplate, err)
	}

	return &Template{
		name:       name,
		source:     source,
		normalized: normalized,
		funcs:      cloneFuncMap(funcs),
	}, nil
}

// Validate checks whether a format template can be normalized and parsed.
func Validate(source string, funcs texttemplate.FuncMap) error {
	_, err := Parse("validate", source, funcs)
	return err
}

// Rewrite rewrites bare keypath actions into explicit httd function calls.
func Rewrite(source string, funcs texttemplate.FuncMap) (string, error) {
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

// Render parses and executes a format template in one step.
func Render(name string, source string, data Getter, funcs texttemplate.FuncMap) (string, error) {
	tmpl, err := Parse(name, source, funcs)
	if err != nil {
		return "", err
	}

	return tmpl.ExecuteString(data)
}

// Execute renders a parsed format template.
func (template *Template) Execute(writer io.Writer, data Getter) error {
	compiled, err := buildTemplate(template.name, template.normalized, template.funcs, data)
	if err != nil {
		return err
	}

	if err = compiled.Execute(writer, data); err != nil {
		return fmt.Errorf("failed executing format template %q: %w", template.name, err)
	}

	return nil
}

// ExecuteString renders a parsed format template into a string.
func (template *Template) ExecuteString(data Getter) (string, error) {
	var buffer bytes.Buffer

	if err := template.Execute(&buffer, data); err != nil {
		return "", err
	}

	return buffer.String(), nil
}

// Source returns the original template source string.
func (template *Template) Source() string {
	return template.source
}

// Normalized returns the rewritten template source string.
func (template *Template) Normalized() string {
	return template.normalized
}

func buildTemplate(name string, source string, funcs texttemplate.FuncMap, data Getter) (*texttemplate.Template, error) {
	compiledFuncs := cloneFuncMap(funcs)
	compiledFuncs["httd"] = func(keypaths string) any {
		return lookup(data, keypaths)
	}

	tmpl := texttemplate.New(name).Delims("${", "}").Funcs(compiledFuncs)

	parsed, err := tmpl.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("failed parsing format template %q: %w", name, err)
	}

	return parsed, nil
}

func cloneFuncMap(funcs texttemplate.FuncMap) texttemplate.FuncMap {
	cloned := make(texttemplate.FuncMap, len(funcs)+1)
	for key, value := range funcs {
		cloned[key] = value
	}
	return cloned
}

func lookup(data Getter, keypaths string) any {
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

	return nil, fmt.Errorf("%w: missing closing } in %q", ErrUnterminatedAction, source[start:])
}

func splitWhitespace(source string) (string, string, string) {
	left := 0
	for left < len(source) {
		r, size := utf8DecodeRuneInString(source[left:])
		if !unicode.IsSpace(r) {
			break
		}
		left += size
	}

	right := len(source)
	for right > left {
		r, size := utf8DecodeLastRuneInString(source[:right])
		if !unicode.IsSpace(r) {
			break
		}
		right -= size
	}

	return source[:left], source[left:right], source[right:]
}

func rewriteAction(source string, funcs texttemplate.FuncMap) (string, error) {
	body := strings.TrimSpace(source)
	if body == "" {
		return "", nil
	}

	if strings.HasPrefix(body, "/*") {
		return "", fmt.Errorf("%w: %s: %q", ErrInvalidTemplate, ErrTemplateComment, body)
	}

	token := firstToken(body)
	if _, reserved := reservedActionKeywords[token]; reserved {
		return "", fmt.Errorf("%w: %s: %q", ErrInvalidTemplate, ErrReservedAction, token)
	}

	if isReservedFunction(token, funcs) {
		return body, nil
	}

	head, tail := splitPipeline(body)
	head = strings.TrimSpace(head)
	if head == "" {
		return "", fmt.Errorf("%w: empty action", ErrInvalidTemplate)
	}

	rewritten := fmt.Sprintf("httd %s", strconv.Quote(head))
	if tail == "" {
		return rewritten, nil
	}

	return rewritten + " " + strings.TrimLeftFunc(tail, unicode.IsSpace), nil
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

func isReservedFunction(token string, funcs texttemplate.FuncMap) bool {
	if token == "httd" {
		return true
	}

	if _, ok := builtinFuncNames[token]; ok {
		return true
	}

	if _, ok := funcs[token]; ok {
		return true
	}

	return false
}

func utf8DecodeRuneInString(source string) (rune, int) {
	return utf8.DecodeRuneInString(source)
}

func utf8DecodeLastRuneInString(source string) (rune, int) {
	return utf8.DecodeLastRuneInString(source)
}
