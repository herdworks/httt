package hypertemplates

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// HyperTemplate Parameters
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
type Parameters []*Parameter

type Operator string

const (
	OperatorEquals     Operator = "=="
	OperatorNotEquals  Operator = "!="
	OperatorRegexMatch Operator = "~="
)

func (parameter *Parameter) Parse(attr string) {
	var ti int = max(strings.Index(attr, ":"), 0) // ti = target index
	var ci int                                    // ci = condition index
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
		parameter.Target = fmt.Sprintf("data-attr-%v", parameter.Number) // TODO: any possible side effects?
	}
	if ci == len(attr) {
		return // early return (no conditions)
	}
	switch parameter.Operator {
	case OperatorRegexMatch:
		parameter.Conditions = []string{attr[min(ci+2, len(attr)):]} // NOTE: regex operator currently only supports one condition
	default:
		parameter.Conditions = strings.Split(attr[min(ci+2, len(attr)):], ",")
	}
}

func (parameter *Parameter) Resolve(data *TemplateData) {
	for _, key := range parameter.Keys {
		var value any = data.Get(key)
		if value == nil {
			continue // guard
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
		return []any{} // guard
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

func (parameter *Parameter) Evaluate() bool {
	if len(parameter.Values) == 0 {
		return false // no values are set
	}
	if len(parameter.Conditions) == 0 {
		return true // a value is set, and no conditions are specified
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
					fmt.Printf("warning: invalid regex condition '%v' in parameter '%v'\n", condition, parameter.Raw)
				}
			}
		}
	}
	return false
}
