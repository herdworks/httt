package value

import (
	"bytes"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"strconv"
	"strings"
	texttemplate "text/template"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	templateName       = "ht-value"
	getFuncName        = "get"
	cssDefaultFuncName = "cssDefault"
)

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
// partial resolution on multi-action templates should give individual actions
// an explicit string-literal fallback.
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

// RenderString parses and executes template values into a string. Used for
// attribute values and other contexts where the rendered output must remain
// inline text. Backed by text/template.
func RenderString(source string, data Getter, syntax Syntax, funcs ...FuncMap) (string, error) {
	parsed, err := parseWithFuncs(source, syntax, mergeFuncMaps(funcs...))
	if err != nil {
		return "", err
	}

	return parsed.executeString(data)
}

// RenderNodes parses and executes template values into a slice of HTML nodes.
// Used for text-node content where the rendered output may include HTML
// produced by funcmap entries returning html/template.HTML. Backed by
// html/template (so user data is auto-escaped, while template.HTML returns
// pass through raw). The rendered string is parsed as a fragment using parent
// as the parsing context, mirroring the html: branch of Template.RenderContent.
//
// A source that renders to plain text returns a single TextNode. A source
// that mixes text and html-returning funcmap entries returns multiple nodes
// (text + element + text, etc.). When any expression in source resolves to
// nil, the original source is restored as a single TextNode (matching
// RenderString's all-or-nothing fallback).
func RenderNodes(source string, data Getter, parent *html.Node, funcs ...FuncMap) ([]*html.Node, error) {
	merged := mergeFuncMaps(funcs...)

	normalized, err := rewriteWithFuncs(source, SyntaxDefault, merged)
	if err != nil {
		return nil, err
	}

	rendered, err := executeHTMLTemplate(normalized, source, data, merged)
	if err != nil {
		return nil, err
	}

	return html.ParseFragment(strings.NewReader(rendered), parent)
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
// bare keypath actions into explicit get function calls. For SyntaxCSS, a
// pre-pass translates --ht-value(...) calls to ${ ... } actions before the main
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
		getFuncName: func(args ...string) any {
			return getFunc(data, args...)
		},
		cssDefaultFuncName: cssDefaultFunc,
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

// executeHTMLTemplate compiles normalized via html/template and executes it
// against data. Mirrors buildTemplate + tmpl.execute but uses html/template
// for context-aware auto-escaping of user data while honoring the
// template.HTML type returned by funcmap entries that produce HTML.
//
// html/template formats nil function returns as the empty string rather than
// "<no value>" (text/template's behavior), so the all-or-nothing source
// restoration is gated on a per-execution miss flag set by the wrapped get
// function. User funcs that return nil are not tracked here; they render as
// empty per html/template's default.
func executeHTMLTemplate(normalized, source string, data Getter, funcs FuncMap) (rendered string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			rendered = ""
			err = fmt.Errorf("failed configuring template value %q: %v", templateName, recovered)
		}
	}()

	var missed bool
	compiledFuncs := htmltemplate.FuncMap{
		getFuncName: func(args ...string) any {
			value := getFunc(data, args...)
			if value == nil {
				missed = true
			}
			return value
		},
		cssDefaultFuncName: cssDefaultFunc,
	}
	for name, fn := range funcs {
		if _, reserved := compiledFuncs[name]; reserved {
			return "", fmt.Errorf("template value function %q is reserved", name)
		}
		compiledFuncs[name] = fn
	}

	compiled, err := htmltemplate.New(templateName).Delims("${", "}").Funcs(compiledFuncs).Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("failed parsing template value %q: %w", templateName, err)
	}

	var buffer bytes.Buffer
	if err := compiled.Execute(&buffer, data); err != nil {
		return "", fmt.Errorf("failed executing template value %q: %w", templateName, err)
	}

	if missed {
		return source, nil
	}

	return buffer.String(), nil
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

// getFunc is registered in the template func map as "get" and resolves
// comma-separated keypaths against the Getter. If more than one argument is
// provided, the final argument is a string fallback returned when lookup misses
// or resolves to an empty string.
func getFunc(data Getter, args ...string) any {
	if len(args) == 0 {
		return nil
	}

	fallback := ""
	hasFallback := len(args) > 1
	if hasFallback {
		fallback = args[len(args)-1]
		args = args[:len(args)-1]
	}

	if data != nil {
		for _, keypaths := range args {
			for _, keypath := range strings.Split(keypaths, ",") {
				keypath = strings.TrimSpace(keypath)
				if keypath == "" {
					continue
				}

				value := data.Get(keypath)
				if value == nil {
					continue
				}
				if s, ok := value.(string); ok && s == "" && hasFallback {
					return fallback
				}
				return value
			}
		}
	}

	if hasFallback {
		return fallback
	}

	return nil
}

func cssDefaultFunc(fallback string, value any) any {
	if value == nil {
		return fallback
	}
	if s, ok := value.(string); ok && s == "<no value>" {
		return fallback
	}
	return value
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

	rewrittenTail, err := rewritePipelineTail(tail, funcs)
	if err != nil {
		return "", err
	}

	if firstToken(head) == getFuncName {
		return appendPipelineTail(head, rewrittenTail), nil
	}

	if registeredUserFunction(firstToken(head), funcs) {
		rewritten, err := rewriteRegisteredFunctionCommand(head, funcs)
		if err != nil {
			return "", err
		}
		return appendPipelineTail(rewritten, rewrittenTail), nil
	}

	if isFunctionCallHead(head) {
		return "", fmt.Errorf("%w: function %q not registered", errInvalidValue, firstToken(head))
	}

	segments := splitCommaSeparated(head)
	if len(segments) == 0 {
		return "", fmt.Errorf("%w: empty action", errInvalidValue)
	}

	fallback := ""
	hasFallback := false
	last := strings.TrimSpace(segments[len(segments)-1])
	if value, ok := stringLiteralValue(last); ok {
		if len(segments) == 1 {
			return "", fmt.Errorf("%w: %w: fallback requires a keypath", errInvalidValue, errInvalidKeypath)
		}
		fallback = value
		hasFallback = true
		segments = segments[:len(segments)-1]
	}

	keypaths := make([]string, 0, len(segments))
	for _, segment := range segments {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			return "", fmt.Errorf("%w: %w: empty keypath in %q", errInvalidValue, errInvalidKeypath, head)
		}
		if _, ok := stringLiteralValue(trimmed); ok {
			return "", fmt.Errorf("%w: %w: string literal fallback must be the last argument", errInvalidValue, errInvalidKeypath)
		}
		if strings.ContainsFunc(trimmed, unicode.IsSpace) {
			return "", fmt.Errorf("%w: %w: %q", errInvalidValue, errInvalidKeypath, trimmed)
		}
		keypaths = append(keypaths, trimmed)
	}

	rewritten := fmt.Sprintf("%s %s", getFuncName, strconv.Quote(strings.Join(keypaths, ", ")))
	if hasFallback {
		rewritten += " " + strconv.Quote(fallback)
	}
	if tail == "" {
		return rewritten, nil
	}

	return appendPipelineTail(rewritten, rewrittenTail), nil
}

func appendPipelineTail(head string, tail string) string {
	if tail == "" {
		return head
	}
	return head + " " + strings.TrimLeftFunc(tail, unicode.IsSpace)
}

func rewritePipelineTail(tail string, funcs FuncMap) (string, error) {
	if tail == "" {
		return "", nil
	}

	var builder strings.Builder
	cursor := 0
	for cursor < len(tail) {
		if tail[cursor] != '|' {
			builder.WriteByte(tail[cursor])
			cursor += 1
			continue
		}

		builder.WriteByte('|')
		cursor += 1
		next := nextTopLevelPipe(tail, cursor)
		segment, err := rewritePipelineSegment(tail[cursor:next], funcs)
		if err != nil {
			return "", err
		}
		builder.WriteString(segment)
		cursor = next
	}

	return builder.String(), nil
}

func rewritePipelineSegment(segment string, funcs FuncMap) (string, error) {
	leading := len(segment) - len(strings.TrimLeftFunc(segment, unicode.IsSpace))
	trailing := len(strings.TrimRightFunc(segment, unicode.IsSpace))
	command := segment[leading:trailing]
	if command == "" {
		return segment, nil
	}
	name := firstToken(command)
	if !registeredFunctionName(name, funcs) {
		return "", fmt.Errorf("%w: function %q not registered", errInvalidValue, name)
	}
	if !registeredUserFunction(name, funcs) {
		return segment, nil
	}

	rewritten, err := rewriteRegisteredFunctionCommand(command, funcs)
	if err != nil {
		return "", err
	}
	return segment[:leading] + rewritten + segment[trailing:], nil
}

func rewriteRegisteredFunctionCommand(command string, funcs FuncMap) (string, error) {
	fields := commandFields(command)
	if len(fields) == 0 {
		return command, nil
	}
	if !registeredUserFunction(command[fields[0].start:fields[0].end], funcs) {
		return command, nil
	}

	var builder strings.Builder
	cursor := 0
	for index, field := range fields {
		builder.WriteString(command[cursor:field.start])
		token := command[field.start:field.end]
		if index > 0 && shouldDotPrefixArgument(token, funcs) {
			builder.WriteByte('.')
		}
		builder.WriteString(token)
		cursor = field.end
	}
	builder.WriteString(command[cursor:])
	return builder.String(), nil
}

type commandField struct {
	start int
	end   int
}

func commandFields(source string) []commandField {
	var fields []commandField
	cursor := 0
	for cursor < len(source) {
		for cursor < len(source) {
			r, size := utf8.DecodeRuneInString(source[cursor:])
			if !unicode.IsSpace(r) {
				break
			}
			cursor += size
		}
		if cursor >= len(source) {
			break
		}

		start := cursor
		cursor = scanCommandField(source, cursor)
		fields = append(fields, commandField{start: start, end: cursor})
	}
	return fields
}

func scanCommandField(source string, cursor int) int {
	var quote byte
	escaped := false
	inComment := false
	depth := 0

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
			cursor += 1
			if char == '`' {
				quote = 0
			}
			continue
		case '"', '\'':
			cursor += 1
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

		if char == '/' && cursor+1 < len(source) && source[cursor+1] == '*' {
			inComment = true
			cursor += 2
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
		default:
			if depth == 0 {
				r, size := utf8.DecodeRuneInString(source[cursor:])
				if unicode.IsSpace(r) {
					return cursor
				}
				cursor += size
				continue
			}
		}
		cursor += 1
	}
	return cursor
}

func shouldDotPrefixArgument(token string, funcs FuncMap) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if _, ok := stringLiteralValue(token); ok {
		return false
	}
	if token == "true" || token == "false" || token == "nil" {
		return false
	}
	if isNumericLiteral(token) {
		return false
	}
	if strings.HasPrefix(token, ".") || strings.HasPrefix(token, "$") || strings.HasPrefix(token, "(") {
		return false
	}
	if registeredFunctionName(token, funcs) {
		return false
	}
	return isIdentifierPath(token)
}

func isNumericLiteral(token string) bool {
	if _, err := strconv.ParseInt(token, 0, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseUint(token, 0, 64); err == nil {
		return true
	}
	if _, err := strconv.ParseFloat(token, 64); err == nil {
		return true
	}
	return false
}

func isIdentifierPath(token string) bool {
	segments := strings.Split(token, ".")
	for _, segment := range segments {
		if !isIdentifier(segment) {
			return false
		}
	}
	return true
}

func isIdentifier(token string) bool {
	if token == "" {
		return false
	}
	for index, r := range token {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func nextTopLevelPipe(source string, cursor int) int {
	var quote byte
	escaped := false
	inComment := false
	depth := 0

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
		case '(':
			depth += 1
		case ')':
			if depth > 0 {
				depth -= 1
			}
		case '|':
			if depth == 0 {
				return cursor
			}
		}
		cursor += 1
	}
	return cursor
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

func splitCommaSeparated(source string) []string {
	var segments []string
	var quote byte
	escaped := false
	inComment := false
	depth := 0
	start := 0

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
		case ',':
			if depth == 0 {
				segments = append(segments, source[start:index])
				start = index + 1
			}
		}
	}

	return append(segments, source[start:])
}

func stringLiteralValue(source string) (string, bool) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", false
	}
	value, err := strconv.Unquote(source)
	if err != nil {
		return "", false
	}
	return value, true
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

// isFunctionCallHead reports whether head looks like an unregistered function
// call: a bare identifier followed by argument tokens. Used after the
// registered-function checks fail to produce a clearer error than the generic
// "invalid keypath" fallthrough would.
func isFunctionCallHead(head string) bool {
	head = strings.TrimSpace(head)
	name := firstToken(head)
	if !isIdentifier(name) {
		return false
	}
	rest := strings.TrimSpace(head[len(name):])
	return rest != ""
}

func registeredFunctionName(name string, funcs FuncMap) bool {
	switch name {
	case getFuncName, cssDefaultFuncName:
		return true
	}
	_, ok := funcs[name]
	return ok
}

func registeredUserFunction(name string, funcs FuncMap) bool {
	_, ok := funcs[name]
	return ok
}
