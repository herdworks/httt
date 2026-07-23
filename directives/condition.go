package directives

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/variable"
	"golang.org/x/net/html"
)

type ConditionalDirective struct{}

var _ Directive = ConditionalDirective{} // make the compiler check Directive interface conformance

type conditionalStatement struct {
	source   string
	lhs      string
	operator conditionalOperator
	rhs      string
}

type conditionalOperator string

const (
	conditionalNot           conditionalOperator = "!"
	conditionalEquals        conditionalOperator = "=="
	conditionalNotEquals     conditionalOperator = "!="
	conditionalRegexMatch    conditionalOperator = "=~"
	conditionalRegexNotMatch conditionalOperator = "!~"
	conditionalIn            conditionalOperator = "in"
	conditionalNotIn         conditionalOperator = "not in"
)

func (d ConditionalDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-if"}
	if !strict {
		attributes = append(attributes, "ht-if")
	}
	return attributes
}
func (d ConditionalDirective) Selectors(strict bool) []string {
	return defaultDirectiveSelectors(d, strict)
}
func (d ConditionalDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}
func (d ConditionalDirective) Process(document *htdocument.Document, element *html.Node, data *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) (err error) {
	if document == nil {
		return nil
	}

	attributes := htdocument.GetAttributes(element, d.Attributes(document.StrictMode)...)
	if len(attributes) == 0 {
		return nil
	}

	funcs := document.Plugins.FuncMap()
	for _, attribute := range attributes {
		if strings.TrimSpace(attribute.Val) == "" {
			return wrapConditionalError(attribute, element, errors.New("ht-if directives require a non-empty conditional expression"))
		}
		var statement *conditionalStatement
		expression := attribute.Val
		for {
			// iterate over condition expression statements
			if expression, statement, err = nextConditionalStatement(expression); err != nil {
				return wrapConditionalError(attribute, element, err)
			}
			if statement == nil {
				// we have reached the end of the expression and no statements returned true
				// TODO: use ht-hidden and data-ht-hidden for untidy conditional elements
				htdocument.SetAttribute(element, "hidden", "true")
				if document.TidyMode {
					htdocument.RemoveElement(element)
				}
				return nil
			}

			truthy, err := evaluateConditionalStatement(*statement, data, funcs)
			if err != nil {
				return wrapConditionalError(attribute, element, err)
			}
			if truthy {
				break // the condition returned true
			}
		}
	}
	if !document.TidyMode {
		// TODO: use ht-hidden and data-ht-hidden for untidy conditional elements
		htdocument.RemoveAttributes(element, "hidden")
	} else {
		removeConditionalAttributes(element, attributes)
	}
	return nil
}

func nextConditionalStatement(expression string) (string, *conditionalStatement, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return "", nil, nil
	}
	if strings.HasPrefix(expression, "OR") && (len(expression) == len("OR") || conditionalWhitespaceAfter(expression, len("OR"))) {
		expression = strings.TrimSpace(expression[len("OR"):])
		if expression == "" {
			return "", nil, errors.New("conditional OR must be followed by a statement")
		}
		if strings.HasPrefix(expression, "OR") && (len(expression) == len("OR") || conditionalWhitespaceAfter(expression, len("OR"))) {
			return "", nil, errors.New("conditional OR must be followed by a statement")
		}
	}

	for cursor := 0; cursor < len(expression); {
		if strings.HasPrefix(expression[cursor:], "${") {
			end := skipConditionalVariable(expression, cursor)
			if end == 0 {
				break
			}
			cursor = end
			continue
		}
		if expression[cursor] == '\'' || expression[cursor] == '"' {
			end := skipConditionalQuote(expression, cursor)
			if end == 0 {
				break
			}
			cursor = end
			continue
		}
		if expression[cursor] == 'O' && cursor+1 < len(expression) && expression[cursor:cursor+2] == "OR" && conditionalWhitespaceBefore(expression, cursor) && (cursor+2 == len(expression) || conditionalWhitespaceAfter(expression, cursor+2)) {
			statement, err := parseConditionalStatement(expression[:cursor])
			if err != nil {
				return "", nil, err
			}
			return strings.TrimSpace(expression[cursor:]), &statement, nil
		}
		_, size := utf8.DecodeRuneInString(expression[cursor:])
		cursor += size
	}
	statement, err := parseConditionalStatement(expression)
	if err != nil {
		return "", nil, err
	}
	return "", &statement, nil
}

func evaluateConditionalStatement(statement conditionalStatement, data variable.Getter, funcs variable.FuncMap) (bool, error) {
	switch statement.operator {
	case "", conditionalNot:
		value, err := variable.ResolveValue(statement.rhs, data)
		if err != nil {
			return false, err
		}
		statementMatches := htdata.IsTruthy(value)
		if statement.operator == conditionalNot {
			return !statementMatches, nil
		}
		return statementMatches, nil
	default:
		return evaluateConditionalComparison(statement, data, funcs)
	}
}

func evaluateConditionalComparison(statement conditionalStatement, data variable.Getter, funcs variable.FuncMap) (bool, error) {
	switch statement.operator {
	case conditionalIn, conditionalNotIn:
		matched, err := evaluateConditionalMembership(statement.lhs, statement.rhs, data, funcs)
		if err != nil {
			return false, err
		}
		if statement.operator == conditionalNotIn {
			return !matched, nil
		}
		return matched, nil
	}

	leftValue, err := renderConditionalOperand(statement.lhs, data, funcs)
	if err != nil {
		return false, err
	}
	rightValue, err := renderConditionalOperand(statement.rhs, data, funcs)
	if err != nil {
		return false, err
	}

	switch statement.operator {
	case conditionalEquals:
		return leftValue == rightValue, nil
	case conditionalNotEquals:
		return leftValue != rightValue, nil
	case conditionalRegexMatch, conditionalRegexNotMatch:
		if leftValue == "" && rightValue == "" {
			return statement.operator == conditionalRegexNotMatch, nil
		}
		expression, err := regexp.Compile(rightValue)
		if err != nil {
			return false, fmt.Errorf("invalid regular expression %q: %w", rightValue, err)
		}
		matched := expression.MatchString(leftValue)
		if statement.operator == conditionalRegexNotMatch {
			return !matched, nil
		}
		return matched, nil
	default:
		return false, fmt.Errorf("unsupported comparison operator %q", statement.operator)
	}
}

func renderConditionalOperand(source string, data variable.Getter, funcs variable.FuncMap) (string, error) {
	if isTypedConditionalExpression(source) {
		rendered, err := variable.RenderString(source, data, variable.SyntaxDefault, funcs)
		if err == nil {
			return rendered, nil
		}
		var missing *variable.MissingValueError
		if errors.As(err, &missing) {
			return "", nil
		}
		return "", err
	}
	if strings.Contains(source, "${") {
		return "", errors.New("comparison template operands must be complete ${ ... } expressions")
	}
	if isQuotedConditionalLiteral(source) {
		return parseConditionalStringLiteral(source)
	}
	if strings.ContainsAny(source, "\"'") {
		return "", errors.New("comparison quoted literals must be complete strings")
	}
	return source, nil
}

func evaluateConditionalMembership(leftSource, rightSource string, data variable.Getter, funcs variable.FuncMap) (bool, error) {
	if !isTypedConditionalExpression(rightSource) {
		return false, errors.New("the right operand of in must be one typed ${ ... } template variable")
	}
	right, err := variable.ResolveValue(rightSource, data)
	if err != nil {
		return false, err
	}
	if right == nil {
		return false, nil
	}

	rightValue := reflect.ValueOf(right)
	if rightValue.Kind() != reflect.Slice {
		switch rightValue.Kind() {
		case reflect.Array, reflect.Map:
			return false, nil
		default:
			return false, fmt.Errorf("the right operand of in must resolve to a slice, got %T", right)
		}
	}

	leftValues, err := membershipOperandValues(leftSource, data, funcs)
	if err != nil {
		return false, err
	}
	for _, left := range leftValues {
		for index := 0; index < rightValue.Len(); index++ {
			right, err := renderConditionalValue(rightValue.Index(index).Interface())
			if err != nil {
				return false, err
			}
			if left == right {
				return true, nil
			}
		}
	}
	return false, nil
}

func membershipOperandValues(source string, data variable.Getter, funcs variable.FuncMap) ([]string, error) {
	if !isTypedConditionalExpression(source) {
		value, err := renderConditionalOperand(source, data, funcs)
		if err != nil {
			return nil, err
		}
		return []string{value}, nil
	}

	value, err := variable.ResolveValue(source, data)
	if err == nil {
		if value == nil {
			return []string{""}, nil
		}
		resolved := reflect.ValueOf(value)
		if resolved.Kind() == reflect.Slice {
			values := make([]string, 0, resolved.Len())
			for index := 0; index < resolved.Len(); index++ {
				rendered, err := renderConditionalValue(resolved.Index(index).Interface())
				if err != nil {
					return nil, err
				}
				values = append(values, rendered)
			}
			return values, nil
		}
		rendered, err := renderConditionalValue(value)
		if err != nil {
			return nil, err
		}
		return []string{rendered}, nil
	}

	// ResolveValue deliberately rejects document-plugin calls. Existing
	// comparison behavior still permits a plugin on the left operand.
	rendered, err := renderConditionalOperand(source, data, funcs)
	if err != nil {
		return nil, err
	}
	return []string{rendered}, nil
}

func renderConditionalValue(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	if stringValue, ok := value.(string); ok {
		return stringValue, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func parseConditionalStatement(source string) (conditionalStatement, error) {
	statementSource := strings.TrimSpace(source)
	if statementSource == "" {
		return conditionalStatement{}, errors.New("empty conditional statement")
	}

	tokens, err := tokenizeConditionalStatement(statementSource)
	if err != nil {
		return conditionalStatement{}, err
	}
	if len(tokens) == 1 {
		if strings.HasPrefix(tokens[0], "!") {
			operand := strings.TrimPrefix(tokens[0], "!")
			if !isTypedConditionalExpression(operand) {
				return conditionalStatement{}, errors.New("unary ! must negate exactly one typed presence expression")
			}
			return conditionalStatement{source: statementSource, operator: conditionalNot, rhs: operand}, nil
		}
		if !isTypedConditionalExpression(tokens[0]) {
			return conditionalStatement{}, errors.New("ht-if directives require at least one ${ ... } template variable")
		}
		return conditionalStatement{source: statementSource, rhs: tokens[0]}, nil
	}
	if len(tokens) == 3 {
		operator := conditionalOperator(tokens[1])
		switch operator {
		case conditionalEquals, conditionalNotEquals, conditionalRegexMatch, conditionalRegexNotMatch, conditionalIn:
			return conditionalStatement{source: statementSource, lhs: tokens[0], operator: operator, rhs: tokens[2]}, nil
		case "~=":
			return conditionalStatement{}, errors.New("legacy ~= regex operator is not supported; use =~")
		}
	}
	if len(tokens) == 4 && tokens[1] == "not" && tokens[2] == "in" {
		return conditionalStatement{source: statementSource, lhs: tokens[0], operator: conditionalNotIn, rhs: tokens[3]}, nil
	}
	return conditionalStatement{}, errors.New("conditional binary operators require whitespace-separated left and right operands")
}

func tokenizeConditionalStatement(source string) ([]string, error) {
	tokens := make([]string, 0, 3)
	for cursor := 0; cursor < len(source); {
		if conditionalWhitespaceAt(source, cursor) {
			_, size := utf8.DecodeRuneInString(source[cursor:])
			cursor += size
			continue
		}
		start := cursor
		if strings.HasPrefix(source[cursor:], "!${") {
			end := skipConditionalVariable(source, cursor+1)
			if end == 0 {
				return nil, errors.New("unterminated template expression in conditional")
			}
			cursor = end
		} else if strings.HasPrefix(source[cursor:], "${") {
			end := skipConditionalVariable(source, cursor)
			if end == 0 {
				return nil, errors.New("unterminated template expression in conditional")
			}
			cursor = end
		} else if source[cursor] == '\'' || source[cursor] == '"' {
			end := skipConditionalQuote(source, cursor)
			if end == 0 {
				return nil, errors.New("unterminated quoted conditional literal")
			}
			cursor = end
		} else {
			for cursor < len(source) && !conditionalWhitespaceAt(source, cursor) {
				_, size := utf8.DecodeRuneInString(source[cursor:])
				cursor += size
			}
		}
		if cursor < len(source) && !conditionalWhitespaceAt(source, cursor) {
			return nil, errors.New("conditional tokens must be separated by whitespace")
		}
		tokens = append(tokens, source[start:cursor])
	}
	return tokens, nil
}

func conditionalWhitespaceAt(source string, cursor int) bool {
	if cursor < 0 || cursor >= len(source) {
		return false
	}
	runeValue, _ := utf8.DecodeRuneInString(source[cursor:])
	return unicode.IsSpace(runeValue)
}

func isQuotedConditionalLiteral(source string) bool {
	return len(source) >= 2 && (source[0] == '\'' || source[0] == '"') && skipConditionalQuote(source, 0) == len(source)
}

func parseConditionalStringLiteral(source string) (string, error) {
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

func isTypedConditionalExpression(source string) bool {
	if !strings.HasPrefix(source, "${") {
		return false
	}
	return skipConditionalVariable(source, 0) == len(source)
}

func skipConditionalVariable(source string, start int) int {
	if start < 0 || start+2 > len(source) || source[start:start+2] != "${" {
		return 0
	}
	quote := byte(0)
	escaped := false
	for cursor := start + 2; cursor < len(source); cursor++ {
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
		if char == '}' {
			return cursor + 1
		}
	}
	return 0
}

func skipConditionalQuote(source string, start int) int {
	quote := source[start]
	escaped := false
	for cursor := start + 1; cursor < len(source); cursor++ {
		if escaped {
			escaped = false
			continue
		}
		if source[cursor] == '\\' {
			escaped = true
			continue
		}
		if source[cursor] == quote {
			return cursor + 1
		}
	}
	return 0
}

func conditionalWhitespaceBefore(source string, cursor int) bool {
	if cursor == 0 {
		return false
	}
	runeValue, _ := utf8.DecodeLastRuneInString(source[:cursor])
	return unicode.IsSpace(runeValue)
}

func conditionalWhitespaceAfter(source string, cursor int) bool {
	if cursor >= len(source) {
		return false
	}
	runeValue, _ := utf8.DecodeRuneInString(source[cursor:])
	return unicode.IsSpace(runeValue)
}

func removeConditionalAttributes(element *html.Node, attributes []html.Attribute) {
	if element == nil || len(attributes) == 0 {
		return
	}
	names := map[string]bool{}
	for _, attribute := range attributes {
		names[strings.ToLower(attribute.Key)] = true
	}
	filtered := element.Attr[:0]
	for _, attribute := range element.Attr {
		if !names[strings.ToLower(attribute.Key)] {
			filtered = append(filtered, attribute)
		}
	}
	element.Attr = filtered
}

func wrapConditionalError(attribute html.Attribute, element *html.Node, err error) error {
	return fmt.Errorf("failed evaluating directive attribute %q=%q:\n%s\n%w", attribute.Key, attribute.Val, htdocument.ElementString(element), err)
}
