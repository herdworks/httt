package variable

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	texttemplate "text/template"
	"unicode"

	"golang.org/x/net/html"
)

// Getter resolves HyperTemplates-style keypaths.
type Getter interface {
	Get(string) any
}

// FuncMap registers user-defined template functions. It is an alias of
// text/template.FuncMap so callers can pass ordinary template functions
// directly.
type FuncMap = texttemplate.FuncMap

// ResolveValue resolves exactly one default-syntax get expression. The
// expression may use either implicit get syntax or get(...), but every
// argument must be a keypath. It returns the first non-empty template-data
// value without rendering or converting it to a string.
func ResolveValue(source string, data Getter) (any, error) {
	expression, err := parseTypedExpression(source)
	if err != nil {
		return nil, err
	}

	return expression.resolveArguments(data), nil
}

// RenderString evaluates template variables and interpolates their values into
// a string. Strings remain unquoted; every other non-nil value is JSON encoded.
// A nil expression result returns MissingValueError.
func RenderString(source string, data Getter, syntax Syntax, funcs ...FuncMap) (string, error) {
	functions := mergeFuncMaps(funcs...)
	segments, err := parse(source, syntax, functions)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	for _, segment := range segments {
		builder.WriteString(segment.literal)
		if segment.expression == nil {
			continue
		}

		value, err := segment.expression.evaluate(data, functions)
		if err != nil {
			return "", err
		}
		if nilValue(value) && segment.expression.cssFallback != nil {
			value = *segment.expression.cssFallback
		}
		if nilValue(value) {
			return "", &MissingValueError{Source: source}
		}

		rendered, err := stringify(value)
		if err != nil {
			return "", fmt.Errorf("failed encoding variable result from %q: %w", source, err)
		}
		builder.WriteString(rendered)
	}

	return builder.String(), nil
}

// RenderNodes evaluates template variables and parses the result as an HTML
// fragment using parent as the parsing context.
func RenderNodes(source string, data Getter, parent *html.Node, funcs ...FuncMap) ([]*html.Node, error) {
	rendered, err := RenderString(source, data, SyntaxDefault, funcs...)
	if err != nil {
		return nil, err
	}

	return html.ParseFragment(strings.NewReader(rendered), parent)
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

func nilValue(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return reflected.IsNil()
	default:
		return false
	}
}

// emptyValue reports whether a value obtained from template data is absent for
// fallback selection. Explicit literals intentionally use nilValue instead so
// a final "" fallback remains a valid author-provided value.
func emptyValue(value any) bool {
	if nilValue(value) {
		return true
	}
	stringValue, ok := value.(string)
	return ok && stringValue == ""
}

func stringify(value any) (string, error) {
	reflected := reflect.ValueOf(value)
	if _, number := value.(json.Number); !number && reflected.Kind() == reflect.String {
		return reflected.String(), nil
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func trimLeftWhitespace(source string) string {
	return strings.TrimLeftFunc(source, unicode.IsSpace)
}

func trimRightWhitespace(source string) string {
	return strings.TrimRightFunc(source, unicode.IsSpace)
}
