package variable

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	texttemplate "text/template"
	"unicode"
	"unicode/utf8"
)

type expression struct {
	name        string
	arguments   []argument
	options     map[string]any
	cssFallback *string
}

type argument struct {
	keypath   string
	literal   any
	isLiteral bool
}

type parsedArguments struct {
	arguments []argument
	options   map[string]any
}

type segment struct {
	literal    string
	expression *expression
}

type variable struct {
	body      string
	end       int
	leftTrim  bool
	rightTrim bool
}

const (
	functionExecutorName = "ht-variable-function"
	getFuncName          = "get"
	textFuncName         = "text"
	markdownFuncName     = "markdown"
	argumentFuncName     = "__ht_argument"
	captureFuncName      = "__ht_capture"
)

func (expr *expression) evaluate(data Getter, funcs FuncMap) (any, error) {
	switch expr.name {
	case getFuncName:
		if expr.options != nil {
			return nil, optionsNotSupportedError(expr.name)
		}
		return expr.resolveArguments(data), nil
	case textFuncName:
		if expr.options != nil {
			return nil, optionsNotSupportedError(expr.name)
		}
		value := expr.resolveArguments(data)
		if nilValue(value) {
			return nil, nil
		}
		return executeFunction(expr.name, []any{value}, funcs)
	case markdownFuncName:
		value := expr.resolveArguments(data)
		if nilValue(value) {
			return nil, nil
		}
		if expr.options != nil {
			return executeFunction(expr.name, []any{value, expr.options}, funcs)
		}
		return executeFunction(expr.name, []any{value}, funcs)
	default:
		arguments := make([]any, 0, len(expr.arguments))
		for _, argument := range expr.arguments {
			arguments = append(arguments, argument.resolve(data))
		}
		if expr.options != nil {
			arguments = []any{map[string]any{
				"args":    arguments,
				"options": expr.options,
			}}
		}
		return executeFunction(expr.name, arguments, funcs)
	}
}

func (expr *expression) resolveArguments(data Getter) any {
	for _, argument := range expr.arguments {
		value := argument.resolve(data)
		if argument.isLiteral {
			return value
		}
		if !emptyValue(value) {
			return value
		}
	}
	return nil
}

func (arg argument) resolve(data Getter) any {
	if arg.isLiteral {
		return arg.literal
	}
	if data == nil {
		return nil
	}
	return data.Get(arg.keypath)
}

func parse(source string, syntax Syntax, funcs FuncMap) ([]segment, error) {
	switch syntax {
	case SyntaxDefault:
		return parseDefault(source, funcs)
	case SyntaxCSS:
		return parseCSS(source, funcs)
	default:
		return nil, fmt.Errorf("%w: unknown syntax %d", errInvalidVariable, syntax)
	}
}

func parseDefault(source string, funcs FuncMap) ([]segment, error) {
	var segments []segment
	cursor := 0
	trimNext := false

	for cursor < len(source) {
		offset := strings.Index(source[cursor:], "${")
		if offset < 0 {
			literal := source[cursor:]
			if trimNext {
				literal = trimLeftWhitespace(literal)
			}
			segments = append(segments, segment{literal: literal})
			break
		}

		start := cursor + offset
		literal := source[cursor:start]
		if trimNext {
			literal = trimLeftWhitespace(literal)
			trimNext = false
		}

		variable, err := readVariable(source, start)
		if err != nil {
			return nil, err
		}
		if variable.leftTrim {
			literal = trimRightWhitespace(literal)
		}

		expression, err := parseExpression(variable.body, funcs)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment{literal: literal, expression: expression})
		trimNext = variable.rightTrim
		cursor = variable.end
	}

	if len(source) == 0 {
		segments = append(segments, segment{})
	}
	return segments, nil
}

func parseCSS(source string, funcs FuncMap) ([]segment, error) {
	var segments []segment
	cursor := 0

	for cursor < len(source) {
		offset := strings.Index(source[cursor:], htVarPrefix)
		if offset < 0 {
			segments = append(segments, segment{literal: source[cursor:]})
			break
		}

		start := cursor + offset
		if !atWordBoundary(source, start) {
			end := start + len(htVarPrefix)
			segments = append(segments, segment{literal: source[cursor:end]})
			cursor = end
			continue
		}

		body, fallback, hasFallback, end, err := readHTVarCall(source, start)
		if err != nil {
			return nil, err
		}
		expression, err := parseExpression(body, funcs)
		if err != nil {
			return nil, err
		}
		if hasFallback {
			expression.cssFallback = &fallback
		}
		segments = append(segments, segment{
			literal:    source[cursor:start],
			expression: expression,
		})
		cursor = end
	}

	if len(source) == 0 {
		segments = append(segments, segment{})
	}
	return segments, nil
}

func parseTypedExpression(source string) (*expression, error) {
	if !strings.HasPrefix(source, "${") {
		return nil, typedVariableError(source, "expected exactly one ${ ... } variable")
	}
	variable, err := readVariable(source, 0)
	if err != nil {
		return nil, typedVariableError(source, err.Error())
	}
	if variable.end != len(source) {
		return nil, typedVariableError(source, "surrounding literal text is not allowed")
	}
	if variable.leftTrim || variable.rightTrim {
		return nil, typedVariableError(source, "trim markers are not allowed in typed variables")
	}

	expression, err := parseExpression(variable.body, nil)
	if err != nil {
		return nil, typedVariableError(source, err.Error())
	}
	if expression.name != getFuncName {
		return nil, typedVariableError(source, "only get expressions are allowed")
	}
	for _, argument := range expression.arguments {
		if argument.isLiteral {
			return nil, typedVariableError(source, "literal operands are not allowed")
		}
	}
	return expression, nil
}

func parseExpression(source string, funcs FuncMap) (*expression, error) {
	body := strings.TrimSpace(source)
	if body == "" {
		return nil, fmt.Errorf("%w: empty variable expression", errInvalidVariable)
	}
	if strings.HasPrefix(body, "/*") {
		return nil, fmt.Errorf("%w: %w: %q", errInvalidVariable, errUnsupportedComment, body)
	}
	if containsUnquotedPipe(body) {
		return nil, fmt.Errorf("%w: pipes are not supported; use function(args) syntax", errInvalidVariable)
	}

	name, argumentsSource, call, err := parseFunctionCall(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errInvalidVariable, err)
	}
	if !call {
		// implicit get expression
		parsed, err := parseArguments(body)
		if err != nil {
			return nil, err
		}
		if parsed.options != nil {
			return nil, optionsNotSupportedError(getFuncName)
		}
		expression := &expression{name: getFuncName, arguments: parsed.arguments}
		if err := validateFallbackArguments(expression); err != nil {
			return nil, err
		}
		return expression, nil
	}

	if name != getFuncName {
		if _, ok := funcs[name]; !ok {
			return nil, fmt.Errorf("%w: function %q not registered", errInvalidVariable, name)
		}
	}
	parsed, err := parseArguments(argumentsSource)
	if err != nil {
		return nil, err
	}
	if name == getFuncName && parsed.options != nil {
		// explicit get(...) does not support options
		return nil, optionsNotSupportedError(name)
	}
	expression := &expression{name: name, arguments: parsed.arguments, options: parsed.options}
	if name == getFuncName || name == textFuncName || name == markdownFuncName {
		if err := validateFallbackArguments(expression); err != nil {
			return nil, err
		}
	}
	return expression, nil
}

func parseFunctionCall(source string) (string, string, bool, error) {
	cursor := 0
	for cursor < len(source) {
		r, size := utf8.DecodeRuneInString(source[cursor:])
		if cursor == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return "", "", false, nil
			}
		} else if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			break
		}
		cursor += size
	}
	name := source[:cursor]
	for cursor < len(source) {
		r, size := utf8.DecodeRuneInString(source[cursor:])
		if !unicode.IsSpace(r) {
			break
		}
		cursor += size
	}
	if cursor >= len(source) || source[cursor] != '(' {
		return "", "", false, nil
	}

	close, err := matchingCallParen(source, cursor)
	if err != nil {
		return "", "", false, err
	}
	if strings.TrimSpace(source[close+1:]) != "" {
		return "", "", false, errors.New("trailing tokens after function call")
	}
	return name, source[cursor+1 : close], true, nil
}

func matchingCallParen(source string, open int) (int, error) {
	quote := byte(0)
	escaped := false
	depth := 0
	for cursor := open; cursor < len(source); cursor++ {
		char := source[cursor]
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
		switch char {
		case '\'', '"':
			quote = char
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return cursor, nil
			}
		}
	}
	return 0, errors.New("missing closing )")
}

func parseArguments(source string) (parsedArguments, error) {
	if strings.TrimSpace(source) == "" {
		return parsedArguments{}, nil
	}
	segments, err := splitArguments(source)
	if err != nil {
		return parsedArguments{}, fmt.Errorf("%w: %v", errInvalidVariable, err)
	}
	parsed := parsedArguments{arguments: make([]argument, 0, len(segments))}
	for index, segment := range segments {
		segment = strings.TrimSpace(segment)
		if strings.HasPrefix(segment, "{") {
			if index != len(segments)-1 {
				return parsedArguments{}, fmt.Errorf("%w: options map must be the final argument", errInvalidVariable)
			}
			if err := json.Unmarshal([]byte(segment), &parsed.options); err != nil {
				return parsedArguments{}, fmt.Errorf("%w: invalid options map: %w", errInvalidVariable, err)
			}
			continue
		}
		argument, err := parseArgument(segment)
		if err != nil {
			return parsedArguments{}, err
		}
		parsed.arguments = append(parsed.arguments, argument)
	}
	return parsed, nil
}

func splitArguments(source string) ([]string, error) {
	var segments []string
	start := 0
	quote := byte(0)
	escaped := false
	parenDepth := 0
	objectDepth := 0
	arrayDepth := 0
	for cursor := 0; cursor < len(source); cursor++ {
		char := source[cursor]
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
		switch char {
		case '\'', '"':
			quote = char
		case '(':
			parenDepth++
		case ')':
			if parenDepth == 0 {
				return nil, errors.New("unexpected )")
			}
			parenDepth--
		case '{':
			objectDepth++
		case '}':
			if objectDepth == 0 {
				return nil, errors.New("unexpected }")
			}
			objectDepth--
		case '[':
			arrayDepth++
		case ']':
			if arrayDepth == 0 {
				return nil, errors.New("unexpected ]")
			}
			arrayDepth--
		case ',':
			if parenDepth == 0 && objectDepth == 0 && arrayDepth == 0 {
				segment := strings.TrimSpace(source[start:cursor])
				if segment == "" {
					return nil, errors.New("empty argument")
				}
				segments = append(segments, segment)
				start = cursor + 1
			}
		}
	}
	if quote != 0 {
		return nil, errors.New("unterminated string")
	}
	if parenDepth != 0 {
		return nil, errors.New("missing closing )")
	}
	if objectDepth != 0 {
		return nil, errors.New("missing closing }")
	}
	if arrayDepth != 0 {
		return nil, errors.New("missing closing ]")
	}
	last := strings.TrimSpace(source[start:])
	if last == "" {
		return nil, errors.New("trailing comma")
	}
	return append(segments, last), nil
}

func parseArgument(source string) (argument, error) {
	if source == "" {
		return argument{}, fmt.Errorf("%w: empty argument", errInvalidVariable)
	}
	if source[0] == '\'' || source[0] == '"' {
		value, err := parseStringLiteral(source)
		if err != nil {
			return argument{}, fmt.Errorf("%w: %v", errInvalidVariable, err)
		}
		return argument{literal: value, isLiteral: true}, nil
	}
	switch source {
	case "true":
		return argument{literal: true, isLiteral: true}, nil
	case "false":
		return argument{literal: false, isLiteral: true}, nil
	case "null", "undefined", "nil", "NaN", "Infinity", "+Infinity", "-Infinity":
		return argument{}, fmt.Errorf("%w: unsupported literal %q", errInvalidVariable, source)
	}
	if looksNumeric(source) {
		value, err := strconv.ParseFloat(source, 64)
		if err != nil {
			return argument{}, fmt.Errorf("%w: invalid numeric literal %q", errInvalidVariable, source)
		}
		return argument{literal: value, isLiteral: true}, nil
	}
	if err := validateKeypath(source); err != nil {
		return argument{}, fmt.Errorf("%w: %w: %v", errInvalidVariable, errInvalidKeypath, err)
	}
	return argument{keypath: source}, nil
}

func optionsNotSupportedError(name string) error {
	return fmt.Errorf("%w: function %q does not support options", errInvalidVariable, name)
}

func parseStringLiteral(source string) (string, error) {
	if len(source) < 2 || source[len(source)-1] != source[0] {
		return "", errors.New("unterminated string")
	}
	encoded := source
	if source[0] == '\'' {
		var builder strings.Builder
		builder.WriteByte('"')
		for cursor := 1; cursor < len(source)-1; cursor++ {
			char := source[cursor]
			switch char {
			case '"':
				builder.WriteString(`\"`)
			case '\\':
				if cursor+1 >= len(source)-1 {
					return "", errors.New("unterminated string escape")
				}
				cursor++
				if source[cursor] == '\'' {
					builder.WriteByte('\'')
					continue
				}
				builder.WriteByte('\\')
				builder.WriteByte(source[cursor])
			default:
				builder.WriteByte(char)
			}
		}
		builder.WriteByte('"')
		encoded = builder.String()
	}

	var value string
	if err := json.Unmarshal([]byte(encoded), &value); err != nil {
		return "", fmt.Errorf("invalid string literal: %w", err)
	}
	return value, nil
}

func validateFallbackArguments(expression *expression) error {
	if len(expression.arguments) == 0 {
		return fmt.Errorf("%w: %w: at least one keypath is required", errInvalidVariable, errInvalidKeypath)
	}
	seenLiteral := false
	keypaths := 0
	for index, argument := range expression.arguments {
		if argument.isLiteral {
			if seenLiteral || index != len(expression.arguments)-1 {
				return fmt.Errorf("%w: %w: literal fallback must be the final argument", errInvalidVariable, errInvalidKeypath)
			}
			seenLiteral = true
			continue
		}
		if seenLiteral {
			return fmt.Errorf("%w: %w: keypath cannot follow a literal fallback", errInvalidVariable, errInvalidKeypath)
		}
		keypaths++
	}
	if keypaths == 0 {
		return fmt.Errorf("%w: %w: fallback requires a keypath", errInvalidVariable, errInvalidKeypath)
	}
	return nil
}

func validateKeypath(source string) error {
	if source == "" {
		return errors.New("empty keypath")
	}
	if strings.ContainsFunc(source, unicode.IsSpace) {
		return fmt.Errorf("keypath contains whitespace: %q", source)
	}
	if strings.ContainsAny(source, "\"'`,|{}[];$") {
		return fmt.Errorf("invalid keypath %q", source)
	}
	if strings.Contains(source, "/*") || strings.Contains(source, "*/") {
		return fmt.Errorf("%w: %q", errUnsupportedComment, source)
	}
	if strings.HasPrefix(source, ".") || strings.HasSuffix(source, ".") {
		return fmt.Errorf("invalid keypath %q", source)
	}
	for _, segment := range strings.Split(source, ".") {
		if segment == "" {
			return fmt.Errorf("invalid keypath %q", source)
		}
		if strings.ContainsAny(segment, "()") {
			if !strings.HasSuffix(segment, "()") || strings.Count(segment, "(") != 1 || strings.Count(segment, ")") != 1 || segment == "()" {
				return fmt.Errorf("nested functions and grouping are not supported: %q", source)
			}
		}
	}
	return nil
}

func containsUnquotedPipe(source string) bool {
	quote := byte(0)
	escaped := false
	for cursor := 0; cursor < len(source); cursor++ {
		char := source[cursor]
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
		if char == '|' {
			return true
		}
	}
	return false
}

func looksNumeric(source string) bool {
	if source == "" {
		return false
	}
	return source[0] == '+' || source[0] == '-' || source[0] >= '0' && source[0] <= '9'
}

func executeFunction(name string, arguments []any, funcs FuncMap) (result any, err error) {
	function, ok := funcs[name]
	if !ok {
		return nil, fmt.Errorf("%w: function %q not registered", errInvalidVariable, name)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = fmt.Errorf("failed configuring variable function %q: %v", name, recovered)
		}
	}()

	captured := false
	argumentName := availableInternalName(argumentFuncName, name)
	captureName := availableInternalName(captureFuncName, name, argumentName)
	compiledFuncs := texttemplate.FuncMap{
		argumentName: func(index int) any { return arguments[index] },
		captureName: func(value any) string {
			result = value
			captured = true
			return ""
		},
		name: function,
	}
	var source strings.Builder
	source.WriteString("{{ ")
	source.WriteString(captureName)
	source.WriteString(" (")
	source.WriteString(name)
	for index := range arguments {
		source.WriteString(" (")
		source.WriteString(argumentName)
		source.WriteByte(' ')
		source.WriteString(strconv.Itoa(index))
		source.WriteByte(')')
	}
	source.WriteString(") }}")

	compiled, err := texttemplate.New(functionExecutorName).Funcs(compiledFuncs).Parse(source.String())
	if err != nil {
		return nil, fmt.Errorf("failed preparing variable function %q: %w", name, err)
	}
	if err := compiled.Execute(io.Discard, nil); err != nil {
		return nil, fmt.Errorf("failed invoking variable function %q: %w", name, err)
	}
	if !captured {
		return nil, nil
	}
	return result, nil
}

func availableInternalName(base string, unavailable ...string) string {
	name := base
	for suffix := 1; ; suffix++ {
		available := true
		for _, candidate := range unavailable {
			if name == candidate {
				available = false
				break
			}
		}
		if available {
			return name
		}
		name = base + strconv.Itoa(suffix)
	}
}

func readVariable(source string, start int) (*variable, error) {
	cursor := start + len("${")
	leftTrim := false
	if cursor < len(source) && source[cursor] == '-' {
		leftTrim = true
		cursor++
	}

	bodyStart := cursor
	quote := byte(0)
	escaped := false
	braceDepth := 0
	for cursor < len(source) {
		char := source[cursor]
		if quote != 0 {
			if escaped {
				escaped = false
				cursor++
				continue
			}
			if char == '\\' {
				escaped = true
				cursor++
				continue
			}
			if char == quote {
				quote = 0
			}
			cursor++
			continue
		}
		if char == '\'' || char == '"' {
			quote = char
			cursor++
			continue
		}
		if char == '{' {
			braceDepth++
			cursor++
			continue
		}
		if char == '}' && braceDepth > 0 {
			braceDepth--
			cursor++
			continue
		}
		if char == '}' {
			rawBody := source[bodyStart:cursor]
			rightTrim := false
			if strings.HasSuffix(rawBody, "-") {
				rightTrim = true
				rawBody = strings.TrimSuffix(rawBody, "-")
			}
			return &variable{
				body:      strings.TrimSpace(rawBody),
				end:       cursor + 1,
				leftTrim:  leftTrim,
				rightTrim: rightTrim,
			}, nil
		}
		cursor++
	}
	return nil, fmt.Errorf("%w: missing closing } in %q", errUnterminatedVariable, source[start:])
}

func typedVariableError(source string, reason string) error {
	return fmt.Errorf("%w: typed variable %q: %s", errInvalidVariable, source, reason)
}
