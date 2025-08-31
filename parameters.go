package hypertemplates

import (
	"fmt"
	"reflect"
	"strings"
)

// HyperTemplate Parameters
//
// Specification:
//   [target]:[key...]=[condition...]
type Parameter struct {
	Raw        string
	Target     string
	Keys       []string
	Values     []any
	Conditions []string
	Number     int
}
type Parameters []*Parameter

func (parameter *Parameter) Parse(attr string) {
	ti := max(strings.Index(attr, ":"),0) // ti = target index
	ci := strings.Index(attr, "=="); if ci < 0 { ci = len(attr) } // ci = condition index
	parameter.Raw = attr
	parameter.Target = attr[:ti]
	parameter.Keys = strings.Split(strings.TrimPrefix(attr[ti:ci], ":"), ",")
	if len(parameter.Target) == 0 { parameter.Target = fmt.Sprintf("data-attr-%v", parameter.Number) } // TODO: any possible side effects?
	if ci == len(attr) { return } // early return (no conditions)
	parameter.Conditions = strings.Split(attr[min(ci + 2, len(attr)):],",")
}

func (parameter *Parameter) Resolve(data *TemplateData) {
	for _, key := range parameter.Keys {
		var value any = data.Get(key)
		if value == nil { continue } // guard
		parameter.Values = append(parameter.Values, value)
	}
}

func (parameter *Parameter) Value() (any) {
	if len(parameter.Values) == 0 { return nil }
	for _,value := range parameter.Values {
		if value != nil { return value }
	}
	return nil
}

func (parameter *Parameter) ValueSet() (values []any) {
	if parameter.Value() == nil { return []any{} } // guard
	var value reflect.Value = reflect.ValueOf(parameter.Value())
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		for i := range value.Len() { values = append(values, value.Index(i).Interface()) }
		return values
	}
	return []any{parameter.Value()}
}

func (parameter *Parameter) Evaluate() (bool) {
	if len(parameter.Values) == 0 { return false } // no values are set
	if len(parameter.Conditions) == 0 { return true } // a value is set, and no conditions are specified
	for _,condition := range parameter.Conditions {
		for _,value := range parameter.ValueSet() {
			if strings.EqualFold(condition, fmt.Sprintf("%v", value)) { return true }
		}
	}
	return false
}
