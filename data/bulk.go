package data

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
)

// Merge returns a new TemplateData with the receiver as base and other
// overlaid on top. Maps are merged recursively. Scalar and slice values from
// other replace those in the receiver. Use MergeAppend to append slices
// instead.
//
// Merge is non-destructive: neither the receiver nor other is mutated.
func (d TemplateData) Merge(other TemplateData) TemplateData {
	return mergeData(d, other, false)
}

// MergeAppend returns a new TemplateData with the receiver as base and other
// overlaid on top. Maps are merged recursively, and slices of the same type
// are appended (other after receiver). Mismatched-type slices behave like
// Merge: other replaces receiver.
//
// MergeAppend is non-destructive: neither the receiver nor other is mutated.
func (d TemplateData) MergeAppend(other TemplateData) TemplateData {
	return mergeData(d, other, true)
}

// Clone returns a deep copy of the receiver. Nested maps and slices are
// recursively copied so that mutations to the clone do not affect the
// original. Returns nil if the receiver is nil.
func (d TemplateData) Clone() TemplateData {
	if d == nil {
		return nil
	}
	cloned := TemplateData{}
	for k, v := range d {
		cloned[k] = cloneValue(v)
	}
	return cloned
}

// Without returns a copy of the receiver with the named top-level keys
// omitted. Without does not descend into nested maps.
func (d TemplateData) Without(keys ...string) TemplateData {
	out := TemplateData{}
	for k, v := range d {
		if slices.Contains(keys, k) {
			continue
		}
		out[k] = v
	}
	return out
}

// StringifyKeys returns a copy of the receiver with all map[any]any values
// recursively converted to map[string]any with string-coerced keys. This
// normalizes data parsed by libraries that emit yaml.v2 shapes (notably
// goldmark-meta) so that downstream Get and Set calls do not need to handle
// interface-keyed maps. The receiver is not mutated.
func (d TemplateData) StringifyKeys() TemplateData {
	if d == nil {
		return nil
	}
	out := TemplateData{}
	for k, v := range d {
		out[k] = stringifyKeys(v)
	}
	return out
}

func mergeData(base, incoming TemplateData, appendSlices bool) TemplateData {
	if len(base) == 0 {
		return incoming.Clone()
	}
	merged := TemplateData{}
	maps.Copy(merged, base)
	for k, v := range incoming {
		existing, present := merged[k]
		if !present {
			merged[k] = v
			continue
		}
		switch ex := existing.(type) {
		case TemplateData:
			if iv, ok := v.(TemplateData); ok {
				merged[k] = mergeData(ex, iv, appendSlices)
				continue
			}
			if iv, ok := v.(map[string]any); ok {
				merged[k] = mergeData(ex, TemplateData(iv), appendSlices)
				continue
			}
			merged[k] = v
		case map[string]any:
			if iv, ok := v.(TemplateData); ok {
				merged[k] = mergeData(TemplateData(ex), iv, appendSlices)
				continue
			}
			if iv, ok := v.(map[string]any); ok {
				merged[k] = mergeData(TemplateData(ex), TemplateData(iv), appendSlices)
				continue
			}
			merged[k] = v
		default:
			if appendSlices {
				if appended, ok := appendSliceValues(existing, v); ok {
					merged[k] = appended
					continue
				}
			}
			merged[k] = v
		}
	}
	return merged
}

func appendSliceValues(existing, incoming any) (any, bool) {
	ev := reflect.ValueOf(existing)
	iv := reflect.ValueOf(incoming)
	if !ev.IsValid() || !iv.IsValid() {
		return nil, false
	}
	if ev.Kind() != reflect.Slice || iv.Kind() != reflect.Slice {
		return nil, false
	}
	if ev.Type() != iv.Type() {
		return nil, false
	}
	out := reflect.MakeSlice(ev.Type(), 0, ev.Len()+iv.Len())
	out = reflect.AppendSlice(out, ev)
	out = reflect.AppendSlice(out, iv)
	return out.Interface(), true
}

func cloneValue(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case TemplateData:
		return x.Clone()
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, vv := range x {
			m[k] = cloneValue(vv)
		}
		return m
	case map[any]any:
		m := make(map[any]any, len(x))
		for k, vv := range x {
			m[k] = cloneValue(vv)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, vv := range x {
			s[i] = cloneValue(vv)
		}
		return s
	case TemplateDataSet:
		s := make(TemplateDataSet, len(x))
		for i, vv := range x {
			s[i] = vv.Clone()
		}
		return s
	case []TemplateData:
		s := make([]TemplateData, len(x))
		for i, vv := range x {
			s[i] = vv.Clone()
		}
		return s
	case []map[string]any:
		s := make([]map[string]any, len(x))
		for i, vv := range x {
			s[i] = cloneValue(vv).(map[string]any)
		}
		return s
	case []map[any]any:
		s := make([]map[any]any, len(x))
		for i, vv := range x {
			s[i] = cloneValue(vv).(map[any]any)
		}
		return s
	case []string:
		return slices.Clone(x)
	case []int:
		return slices.Clone(x)
	case []float32:
		return slices.Clone(x)
	case []float64:
		return slices.Clone(x)
	}
	return v
}

func stringifyKeys(v any) any {
	switch x := v.(type) {
	case map[any]any:
		m := make(map[string]any, len(x))
		for k, vv := range x {
			m[fmt.Sprintf("%v", k)] = stringifyKeys(vv)
		}
		return m
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, vv := range x {
			m[k] = stringifyKeys(vv)
		}
		return m
	case TemplateData:
		m := TemplateData{}
		for k, vv := range x {
			m[k] = stringifyKeys(vv)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, vv := range x {
			s[i] = stringifyKeys(vv)
		}
		return s
	}
	return v
}
