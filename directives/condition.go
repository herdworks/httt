package directives

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strings"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/variable"
	"golang.org/x/net/html"
)

type ConditionalDirective struct{}

var _ Directive = ConditionalDirective{} // make the compiler check Directive interface conformance
var _ DirectiveExpression = ConditionalDirective{}
var _ IncrementalDirectiveExpression = ConditionalDirective{}

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

var conditionalExpressionGrammar = ExpressionGrammar{
	Operators:           []string{"not in", "in", "==", "!=", "=!", "=~", "~=", "!~", "~!", "!"},
	Separator:           "OR",
	AllowQuotedLiterals: true,
	MaxTokens:           3,
}

type conditionalExpression struct {
	statements [8]conditionalStatement
	overflow   []conditionalStatement
	count      int
}

func (expression *conditionalExpression) append(statement conditionalStatement) {
	if expression.count < len(expression.statements) {
		expression.statements[expression.count] = statement
	} else {
		expression.overflow = append(expression.overflow, statement)
	}
	expression.count++
}

func (expression *conditionalExpression) at(index int) conditionalStatement {
	if index < len(expression.statements) {
		return expression.statements[index]
	}
	return expression.overflow[index-len(expression.statements)]
}

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

func (ConditionalDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

// ParseExpression tokenizes an entire conditional expression. It preserves a
// flat token stream so all OR branches can be validated before evaluation.
func (ConditionalDirective) ParseExpression(source string) ([]Token, error) {
	return tokenizeExpression(source, conditionalExpressionGrammar)
}

// ParseNextExpression tokenizes one OR-bounded conditional statement. It is
// available to callers that need bounded incremental parsing; Process keeps
// the eager whole-expression path to preserve ht-if validation semantics.
func (ConditionalDirective) ParseNextExpression(source string) (remainder string, tokens []Token, err error) {
	return tokenizeNextExpression(source, conditionalExpressionGrammar)
}

// ValidateExpression checks every conditional statement without resolving
// values, rendering plugins, or compiling regular expressions.
func (ConditionalDirective) ValidateExpression(tokens []Token) error {
	return validateConditionalTokenGroups(tokens)
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
		var expression conditionalExpression
		if err := parseConditionalExpression(attribute.Val, &expression); err != nil {
			return wrapConditionalError(attribute, element, err)
		}
		matched := false
		for index := 0; index < expression.count; index++ {
			truthy, err := evaluateConditionalStatement(expression.at(index), data, funcs)
			if err != nil {
				return wrapConditionalError(attribute, element, err)
			}
			if truthy {
				matched = true
				break
			}
		}
		if !matched {
			// TODO: use ht-hidden and data-ht-hidden for untidy conditional elements
			htdocument.SetAttribute(element, "hidden", "true")
			if document.TidyMode {
				htdocument.RemoveElement(element)
			}
			return nil
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

func parseConditionalExpression(source string, expression *conditionalExpression) error {
	tokens, err := (ConditionalDirective{}).ParseExpression(source)
	if err != nil {
		return err
	}
	if err := (ConditionalDirective{}).ValidateExpression(tokens); err != nil {
		return err
	}
	return buildConditionalExpression(source, tokens, expression)
}

func validateConditionalTokenGroups(tokens []Token) error {
	start := 0
	if len(tokens) > 0 && isConditionalSeparator(tokens[0]) {
		start++
	}
	for index := start; index < len(tokens); index++ {
		if !isConditionalSeparator(tokens[index]) {
			continue
		}
		if index == start {
			return errors.New("conditional OR must be followed by a statement")
		}
		if _, err := parseConditionalTokens("", tokens[start:index]); err != nil {
			return err
		}
		start = index + 1
	}
	if start == len(tokens) {
		return errors.New("conditional OR must be followed by a statement")
	}
	_, err := parseConditionalTokens("", tokens[start:])
	return err
}

func buildConditionalExpression(source string, tokens []Token, expression *conditionalExpression) error {
	start := 0
	if len(tokens) > 0 && isConditionalSeparator(tokens[0]) {
		start++
	}
	for index := start; index < len(tokens); index++ {
		if !isConditionalSeparator(tokens[index]) {
			continue
		}
		statement, err := parseConditionalTokenGroup(source, tokens[start:index])
		if err != nil {
			return err
		}
		expression.append(statement)
		start = index + 1
	}
	statement, err := parseConditionalTokenGroup(source, tokens[start:])
	if err != nil {
		return err
	}
	expression.append(statement)
	return nil
}

func isConditionalSeparator(token Token) bool {
	return token.IsDelimiter() && token.Raw == conditionalExpressionGrammar.Separator
}

func parseConditionalTokenGroup(source string, tokens []Token) (conditionalStatement, error) {
	if len(tokens) == 0 {
		return conditionalStatement{}, errors.New("empty conditional statement")
	}
	statementSource := source[tokens[0].Start:tokens[len(tokens)-1].End]
	return parseConditionalTokens(statementSource, tokens)
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

	tokens, err := (ConditionalDirective{}).ParseExpression(statementSource)
	if err != nil {
		return conditionalStatement{}, err
	}
	if len(tokens) == 0 {
		return conditionalStatement{}, errors.New("empty conditional statement")
	}
	if slices.ContainsFunc(tokens, isConditionalSeparator) {
		return conditionalStatement{}, errors.New("conditional statements cannot contain OR")
	}
	return parseConditionalTokens(statementSource, tokens)
}

func parseConditionalTokens(statementSource string, tokens []Token) (conditionalStatement, error) {
	if len(tokens) == 1 {
		if !isTypedConditionalExpression(tokens[0].Raw) {
			return conditionalStatement{}, errors.New("ht-if directives require at least one ${ ... } template variable")
		}
		return conditionalStatement{source: statementSource, rhs: tokens[0].Raw}, nil
	}
	if len(tokens) == 2 && tokens[0].Raw == "!" && tokens[1].IsVariable() {
		return conditionalStatement{source: statementSource, operator: conditionalNot, rhs: tokens[1].Raw}, nil
	}
	if len(tokens) == 3 {
		if !tokens[1].IsOperator() {
			return conditionalStatement{}, errors.New("conditional binary operators require whitespace-separated left and right operands")
		}
		operator := normalizeConditionalOperator(tokens[1].Raw)
		switch operator {
		case conditionalEquals, conditionalNotEquals, conditionalRegexMatch, conditionalRegexNotMatch, conditionalIn:
			return conditionalStatement{source: statementSource, lhs: tokens[0].Raw, operator: operator, rhs: tokens[2].Raw}, nil
		}
	}
	if len(tokens) == 3 && normalizeConditionalOperator(tokens[1].Raw) == conditionalNotIn {
		return conditionalStatement{source: statementSource, lhs: tokens[0].Raw, operator: conditionalNotIn, rhs: tokens[2].Raw}, nil
	}
	return conditionalStatement{}, errors.New("conditional binary operators require whitespace-separated left and right operands")
}

func normalizeConditionalOperator(operator string) conditionalOperator {
	switch operator {
	case "=!":
		return conditionalNotEquals
	case "~=":
		return conditionalRegexMatch
	case "~!":
		return conditionalRegexNotMatch
	default:
		return conditionalOperator(operator)
	}
}

func isQuotedConditionalLiteral(source string) bool {
	if len(source) < 2 || (source[0] != '\'' && source[0] != '"') {
		return false
	}
	end, err := scanQuotedToken(source, 0)
	return err == nil && end == len(source)
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
	end, err := scanVariableToken(source, 0)
	return err == nil && end == len(source)
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
