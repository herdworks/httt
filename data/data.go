// Package data provides the HyperTemplates template data object: a string-keyed
// map[string]any with dot-notation property access.
//
// A template data key is a dot-separated path of property names, where numeric
// segments index into slices and arrays (1-indexed; there is no "zero"
// element). Trailing function-style accessors compute derived values from the
// resolved property:
//
//	site.title
//	page.author.name
//	page.attachments.1.href
//	page.tags.length()
//	page.metadata.keys()
//
// Per the template data contract, missing keys are not errors: Get returns nil
// and Has returns false without penalty.
//
// TemplateData has map[string]any as its underlying type, so callers can move
// values between the two representations with a direct conversion:
//
//	d := data.TemplateData(m)
//	m := map[string]any(d)
//
// JSON and YAML marshal and unmarshal work unchanged.
package data

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// TemplateData is a template data object: a string-keyed map whose values can
// be addressed by dot-notation keys.
type TemplateData map[string]any

// TemplateDataSet is a list of template data objects.
type TemplateDataSet []TemplateData

// Get returns the value of the property at key, or nil if the key does not
// resolve. Missing keys are not errors.
//
// key uses dot notation. Numeric segments index into slices and arrays
// (1-indexed). A trailing function-style segment ("length()", "count()",
// "keys()") computes a derived value from the resolved property.
//
// An empty key returns the receiver as any.
func (d TemplateData) Get(key string) any {
	if key == "" {
		return any(d)
	}
	var v any = d
	for _, seg := range parseKey(key) {
		if seg.fn {
			v = applyFunction(v, seg.name)
			continue
		}
		v = step(v, seg.name)
		if v == nil {
			return nil
		}
	}
	return v
}

// Has reports whether the property at key has a non-empty value.
//
// Has returns true for present scalars (numbers, booleans), non-empty strings,
// and non-empty slices, arrays, and maps. It returns false for missing keys,
// nil values, empty strings, and empty containers.
//
// Has matches the semantics of the ht-if directive: "the property has a usable
// value", not just "the key is present". To distinguish present-nil from
// absent, use Get and inspect the result directly.
func (d TemplateData) Has(key string) bool {
	v := d.Get(key)
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case bool, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	case string:
		return len(x) > 0
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		return rv.Len() > 0
	}
	return false
}

// Set writes value at the property at key, creating intermediate maps as
// needed. Set returns an error if a non-map value already occupies an
// intermediate segment of the path; in that case the existing value is left in
// place. Function-style segments are not valid in Set keys.
func (d TemplateData) Set(key string, value any) error {
	if d == nil {
		return errors.New("data: cannot Set on nil TemplateData")
	}
	if key == "" {
		return errors.New("data: cannot Set with empty key")
	}
	return setSegments(d, strings.Split(key, "."), value)
}

// Delete removes the property at key. If key resolves to a nested map,
// Delete removes the entry from that map. Delete is a no-op for missing keys.
func (d TemplateData) Delete(key string) {
	if key == "" {
		return
	}
	path := strings.Split(key, ".")
	if len(path) == 1 {
		delete(d, key)
		return
	}
	parent := d.Get(strings.Join(path[:len(path)-1], "."))
	last := path[len(path)-1]
	switch x := parent.(type) {
	case TemplateData:
		delete(x, last)
	case map[string]any:
		delete(x, last)
	}
}

// Keys returns the sorted top-level keys of the map at key. If key is empty,
// Keys returns the sorted top-level keys of the receiver. Returns nil if key
// does not resolve to a map.
func (d TemplateData) Keys(key string) []string {
	if key == "" {
		return keysOf(d)
	}
	return keysOf(d.Get(key))
}

// Len returns the length of the value at key. For maps, slices, arrays, and
// strings, Len returns the natural length. For other types, Len returns 0.
// If key is empty, Len returns the length of the receiver.
func (d TemplateData) Len(key string) int {
	if key == "" {
		return lengthOf(d)
	}
	return lengthOf(d.Get(key))
}

// segment represents a single component of a parsed template data key. A
// function segment ("length()") is applied to the current walk value rather
// than used to descend into a nested property.
type segment struct {
	name string
	fn   bool
}

func parseKey(key string) []segment {
	if key == "" {
		return nil
	}
	parts := strings.Split(key, ".")
	segs := make([]segment, len(parts))
	for i, p := range parts {
		if strings.HasSuffix(p, "()") {
			segs[i] = segment{name: strings.TrimSuffix(p, "()"), fn: true}
		} else {
			segs[i] = segment{name: p}
		}
	}
	return segs
}

// step descends into v by one path segment.
func step(v any, seg string) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case TemplateData:
		return x[seg]
	case map[string]any:
		return x[seg]
	case []any:
		return sliceAt(x, seg)
	case TemplateDataSet:
		n, ok := sliceIndex(seg, len(x))
		if !ok {
			return nil
		}
		return x[n]
	case []TemplateData:
		n, ok := sliceIndex(seg, len(x))
		if !ok {
			return nil
		}
		return x[n]
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() == reflect.String {
			mv := rv.MapIndex(reflect.ValueOf(seg))
			if !mv.IsValid() {
				return nil
			}
			return mv.Interface()
		}
		// map[any]any and other interface-keyed maps: stringify keys to compare.
		for _, k := range rv.MapKeys() {
			if fmt.Sprintf("%v", k.Interface()) == seg {
				return rv.MapIndex(k).Interface()
			}
		}
		return nil
	case reflect.Slice, reflect.Array:
		n, ok := sliceIndex(seg, rv.Len())
		if !ok {
			return nil
		}
		return rv.Index(n).Interface()
	}
	return nil
}

// sliceAt returns the 1-indexed element of s identified by seg, or nil if seg
// is non-numeric, zero, or out of range.
func sliceAt[T any](s []T, seg string) any {
	n, ok := sliceIndex(seg, len(s))
	if !ok {
		return nil
	}
	return s[n]
}

// sliceIndex parses seg as a 1-indexed slice index and returns the
// corresponding 0-based index plus an ok flag. Non-numeric segments, zero, and
// out-of-range indices are reported as not-ok.
func sliceIndex(seg string, length int) (int, bool) {
	n, err := strconv.Atoi(seg)
	if err != nil {
		return 0, false
	}
	if n < 1 || n > length {
		return 0, false
	}
	return n - 1, true
}

// applyFunction evaluates a function-style accessor against the current walk
// value. Unknown function names resolve to nil so missing accessors fail
// silently per the template data contract.
func applyFunction(v any, name string) any {
	switch name {
	case "length", "count":
		return lengthOf(v)
	case "keys":
		return keysOf(v)
	}
	return nil
}

func lengthOf(v any) int {
	if v == nil {
		return 0
	}
	switch x := v.(type) {
	case TemplateData:
		return len(x)
	case map[string]any:
		return len(x)
	case string:
		return len(x)
	case []any:
		return len(x)
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
		return rv.Len()
	}
	return 0
}

func keysOf(v any) []string {
	if v == nil {
		return nil
	}
	var keys []string
	switch x := v.(type) {
	case TemplateData:
		keys = make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
	case map[string]any:
		keys = make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Map {
			return nil
		}
		keys = make([]string, 0, rv.Len())
		for _, k := range rv.MapKeys() {
			keys = append(keys, fmt.Sprintf("%v", k.Interface()))
		}
	}
	sort.Strings(keys)
	return keys
}

func setSegments(d TemplateData, path []string, value any) error {
	if len(path) == 0 {
		return nil
	}
	head := path[0]
	rest := path[1:]
	if len(rest) == 0 {
		d[head] = value
		return nil
	}
	existing, present := d[head]
	if !present || existing == nil {
		nested := TemplateData{}
		d[head] = nested
		return setSegments(nested, rest, value)
	}
	switch x := existing.(type) {
	case TemplateData:
		return setSegments(x, rest, value)
	case map[string]any:
		return setSegments(TemplateData(x), rest, value)
	}
	return fmt.Errorf("data: cannot Set %q: a non-map value of type %T already occupies path", strings.Join(path, "."), existing)
}
