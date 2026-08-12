package directives

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/herdworks/httt/data"
)

// Parameter is the legacy HyperTemplates directive parameter.
//
// Specification:
//
//	[target]:[key...]==[condition...]
type Parameter struct {
	Raw        string
	Target     string
	Keys       []string
	Operator   Operator
	Conditions []string
	Values     []any
	Number     int
}

type Parameters []*Parameter // make the compiler check Directive interface conformance

type Operator string

const (
	OperatorEquals     Operator = "=="
	OperatorNotEquals  Operator = "!="
	OperatorRegexMatch Operator = "~="
)

func (parameter *Parameter) Parse(attr string) {
	var ti int = max(strings.Index(attr, ":"), 0)
	var ci int
	switch {
	case strings.Contains(attr, "=="):
		parameter.Operator = OperatorEquals
		ci = strings.Index(attr, "==")
	case strings.Contains(attr, "!="):
		parameter.Operator = OperatorNotEquals
		ci = strings.Index(attr, "!=")
	case strings.Contains(attr, "~="):
		parameter.Operator = OperatorRegexMatch
		ci = strings.Index(attr, "~=")
	default:
		ci = len(attr)
	}
	parameter.Raw = attr
	parameter.Target = attr[:ti]
	parameter.Keys = strings.Split(strings.TrimPrefix(attr[ti:ci], ":"), ",")
	if len(parameter.Target) == 0 {
		parameter.Target = fmt.Sprintf("data-attr-%v", parameter.Number)
	}
	if ci == len(attr) {
		return
	}
	switch parameter.Operator {
	case OperatorRegexMatch:
		parameter.Conditions = []string{attr[min(ci+2, len(attr)):]} // regex operator currently only supports one condition
	default:
		parameter.Conditions = strings.Split(attr[min(ci+2, len(attr)):], ",")
	}
}

func (parameter *Parameter) Resolve(data *data.TemplateData) {
	for _, key := range parameter.Keys {
		var value any = data.Get(key)
		if value == nil {
			continue
		}
		parameter.Values = append(parameter.Values, value)
	}
}

func (parameter *Parameter) Value() any {
	if len(parameter.Values) == 0 {
		return nil
	}
	for _, value := range parameter.Values {
		if value != nil {
			return value
		}
	}
	return nil
}

func (parameter *Parameter) ValueSet() (values []any) {
	if parameter.Value() == nil {
		return []any{}
	}
	var value reflect.Value = reflect.ValueOf(parameter.Value())
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		for i := range value.Len() {
			values = append(values, value.Index(i).Interface())
		}
		return values
	}
	return []any{parameter.Value()}
}

func (parameter *Parameter) Evaluate(w io.Writer) bool {
	if w == nil {
		w = os.Stdout
	}

	if len(parameter.Values) == 0 {
		return false
	}
	if len(parameter.Conditions) == 0 {
		return true
	}
	for _, condition := range parameter.Conditions {
		for _, value := range parameter.ValueSet() {
			switch parameter.Operator {
			case OperatorEquals:
				if strings.EqualFold(condition, fmt.Sprintf("%v", value)) {
					return true
				}
			case OperatorNotEquals:
				if !strings.EqualFold(condition, fmt.Sprintf("%v", value)) {
					return true
				}
			case OperatorRegexMatch:
				if re, err := regexp.Compile(condition); err == nil {
					if re.MatchString(fmt.Sprintf("%v", value)) {
						return true
					}
				} else {
					fmt.Fprintf(w, "warning: invalid regex condition '%v' in parameter '%v'\n", condition, parameter.Raw)
				}
			}
		}
	}
	return false
}

func ParseParameters(attr string, data *data.TemplateData) Parameters {
	params := Parameters{}
	if len(attr) == 0 {
		return params
	}
	for _, param := range strings.Split(attr, ";") {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		params = append(params, ParseParameter(param, data))
	}
	return params
}

func ParseParameter(attr string, data *data.TemplateData) *Parameter {
	param := &Parameter{}
	param.Parse(attr)
	param.Resolve(data)
	return param
}
