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
	"maps"
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

// TemplateDataKeyPath describes a key path for TemplateData mutation
// operations. It models mutation paths as a raw dot-notation string, zero or
// more branch path segments, and a final leaf key. It does not model the full
// read-only Get key language, including computed function-style accessors like
// length(), count(), and keys().
type TemplateDataKeyPath struct {
	// Raw is the original dot-notation key.
	Raw string
	// Path contains the branch segments between the TemplateData root and leaf.
	Path []string
	// Key is the leaf key to read or mutate in the resolved branch map.
	Key string
}

// =============================================================================
// TemplateData methods
// =============================================================================

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
	if !strings.ContainsRune(key, '.') && !strings.HasSuffix(key, "()") {
		return d[key]
	}

	var v any = d
	for {
		segment, remaining, found := strings.Cut(key, ".")
		if strings.HasSuffix(segment, "()") {
			v = applyFunction(v, strings.TrimSuffix(segment, "()"))
		} else {
			v = step(v, segment)
			if v == nil {
				return nil
			}
		}
		if !found {
			return v
		}
		key = remaining
	}
}

// Has reports whether the property at key has a non-empty value.
//
// Has returns true for all numbers (including zero), boolean true, non-empty
// strings and containers, and other non-nil values. It returns false for
// missing or nil values, boolean false, empty strings, and empty slices,
// arrays, and maps.
//
// Has matches the semantics of the ht-if directive: "the property has a usable
// value", not just "the key is present". To distinguish present-nil from
// absent, use Get and inspect the result directly.
func (d TemplateData) Has(key string) bool {
	return IsTruthy(d.Get(key))
}

func (d *TemplateData) ShallowClone() TemplateData {
	if d == nil || *d == nil {
		return TemplateData{}
	}
	return maps.Clone(*d)
}

// IsTruthy reports whether value has the same truthiness used by TemplateData.Has.
// Missing and nil values, false, empty strings, and empty containers are false;
// all numbers, including zero, and non-empty values are true.
func IsTruthy(value any) bool {
	v := value
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool()
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return rv.Len() > 0
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Pointer, reflect.UnsafePointer:
		return !rv.IsNil()
	default:
		return true
	}
}

// Set writes value at the property at key, creating intermediate maps as
// needed. Set returns an error if a non-map value already occupies an
// intermediate segment of the path; in that case the existing value is left in
// place. Mutation paths traverse maps only; function-style segments are not
// valid in Set keys.
func (d TemplateData) Set(key string, value any) error {
	if d == nil {
		return errors.New("data: cannot Set on nil TemplateData")
	}
	var keyPath TemplateDataKeyPath
	if err := keyPath.parse(key); err != nil {
		return fmt.Errorf("data: cannot Set %q: %w", key, err)
	}
	if err := keyPath.isReachable(d); err != nil {
		return fmt.Errorf("data: cannot Set %q: %w", keyPath.Raw, err)
	}
	parent, err := keyPath.resolveBranches(d)
	if err != nil {
		return fmt.Errorf("data: cannot Set %q: %w", keyPath.Raw, err)
	}
	parent[keyPath.Key] = value
	return nil
}

// Replace writes value at key only when the leaf key already exists. Replace
// returns an error if the leaf is missing or a non-map value occupies an
// intermediate segment. Use Set when missing leaf values should be created.
func (d TemplateData) Replace(key string, value any) error {
	if d == nil {
		return errors.New("data: cannot Replace on nil TemplateData")
	}
	var keyPath TemplateDataKeyPath
	if err := keyPath.parse(key); err != nil {
		return fmt.Errorf("data: cannot Replace %q: %w", key, err)
	}
	if err := keyPath.isReachable(d); err != nil {
		return fmt.Errorf("data: cannot Replace %q: %w", keyPath.Raw, err)
	}
	if !keyPath.branchExists(d) {
		return fmt.Errorf("data: cannot Replace %q: branch path does not exist", keyPath.Raw)
	}
	parent, err := keyPath.resolveBranches(d)
	if err != nil {
		return fmt.Errorf("data: cannot Replace %q: %w", keyPath.Raw, err)
	}
	if _, present := parent[keyPath.Key]; !present {
		return fmt.Errorf("data: cannot Replace %q: key does not exist", keyPath.Raw)
	}
	parent[keyPath.Key] = value
	return nil
}

// Add writes value at key, creating intermediate maps as needed. Add returns
// an error if the leaf key already exists or a non-map value occupies an
// intermediate segment. Use Set when overwriting existing leaf values is
// acceptable.
func (d TemplateData) Add(key string, value any) error {
	if d == nil {
		return errors.New("data: cannot Add on nil TemplateData")
	}
	var keyPath TemplateDataKeyPath
	if err := keyPath.parse(key); err != nil {
		return fmt.Errorf("data: cannot Add %q: %w", key, err)
	}
	if err := keyPath.isReachable(d); err != nil {
		return fmt.Errorf("data: cannot Add %q: %w", keyPath.Raw, err)
	}
	parent, err := keyPath.resolveBranches(d)
	if err != nil {
		return fmt.Errorf("data: cannot Add %q: %w", keyPath.Raw, err)
	}
	if _, present := parent[keyPath.Key]; present {
		return fmt.Errorf("data: cannot Add %q: key already exists", keyPath.Raw)
	}
	parent[keyPath.Key] = value
	return nil
}

// Append appends value to the array at key, creating intermediate maps as
// needed. If the leaf is missing, Append creates it as a []any containing value.
// If the leaf exists, it must be a supported JSON-like slice type.
func (d TemplateData) Append(key string, value any) error {
	if d == nil {
		return errors.New("data: cannot Append on nil TemplateData")
	}
	var keyPath TemplateDataKeyPath
	if err := keyPath.parse(key); err != nil {
		return fmt.Errorf("data: cannot Append %q: %w", key, err)
	}
	if err := keyPath.appendable(d); err != nil {
		return fmt.Errorf("data: cannot Append %q: %w", keyPath.Raw, err)
	}
	parent, err := keyPath.resolveBranches(d)
	if err != nil {
		return fmt.Errorf("data: cannot Append %q: %w", keyPath.Raw, err)
	}
	existing, present := parent[keyPath.Key]
	if !present || existing == nil {
		parent[keyPath.Key] = []any{value}
		return nil
	}
	appended, err := appendValue(existing, value)
	if err != nil {
		return fmt.Errorf("data: cannot Append %q: %w", keyPath.Raw, err)
	}
	parent[keyPath.Key] = appended
	return nil
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

// =============================================================================
// TemplateDataKeyPath methods
// =============================================================================

func (kp *TemplateDataKeyPath) parse(raw string) error {
	if kp == nil {
		return errors.New("nil TemplateDataKeyPath")
	}
	if raw == "" {
		return errors.New("empty key path")
	}
	segments := strings.Split(raw, ".")
	for _, segment := range segments {
		if segment == "" {
			return errors.New("empty path segment")
		}
		if strings.HasSuffix(segment, "()") {
			return errors.New("function-style segments are not valid mutation keys")
		}
	}
	kp.Raw = raw
	kp.Path = segments[:len(segments)-1]
	kp.Key = segments[len(segments)-1]
	return nil
}

func (kp TemplateDataKeyPath) isReachable(root TemplateData) error {
	if root == nil {
		return errors.New("nil TemplateData")
	}
	current := root
	for _, branch := range kp.Path {
		existing, present := current[branch]
		if !present || existing == nil {
			return nil
		}
		nested, ok := templateDataMap(existing)
		if !ok {
			return fmt.Errorf("branch %q is %T, not a map", branch, existing)
		}
		current = nested
	}
	return nil
}

func (kp TemplateDataKeyPath) branchExists(root TemplateData) bool {
	_, exists, err := kp.existingBranches(root)
	return err == nil && exists
}

func (kp TemplateDataKeyPath) resolveBranches(root TemplateData) (TemplateData, error) {
	if root == nil {
		return nil, errors.New("nil TemplateData")
	}
	current := root
	for _, branch := range kp.Path {
		existing, present := current[branch]
		if !present || existing == nil {
			nested := TemplateData{}
			current[branch] = nested
			current = nested
			continue
		}
		nested, ok := templateDataMap(existing)
		if !ok {
			return nil, fmt.Errorf("branch %q is %T, not a map", branch, existing)
		}
		current = nested
	}
	return current, nil
}

func (kp TemplateDataKeyPath) appendable(root TemplateData) error {
	if err := kp.isReachable(root); err != nil {
		return err
	}
	parent, exists, err := kp.existingBranches(root)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	existing, present := parent[kp.Key]
	if !present || existing == nil {
		return nil
	}
	if !isAppendableSlice(existing) {
		return fmt.Errorf("target is %T, not a supported JSON-like array", existing)
	}
	return nil
}

func (kp TemplateDataKeyPath) existingBranches(root TemplateData) (TemplateData, bool, error) {
	if root == nil {
		return nil, false, errors.New("nil TemplateData")
	}
	current := root
	for _, branch := range kp.Path {
		existing, present := current[branch]
		if !present || existing == nil {
			return nil, false, nil
		}
		nested, ok := templateDataMap(existing)
		if !ok {
			return nil, false, fmt.Errorf("branch %q is %T, not a map", branch, existing)
		}
		current = nested
	}
	return current, true, nil
}

// =============================================================================
// Helper functions
// =============================================================================

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

func templateDataMap(v any) (TemplateData, bool) {
	switch x := v.(type) {
	case TemplateData:
		return x, true
	case map[string]any:
		return TemplateData(x), true
	}
	return nil, false
}

func isAppendableSlice(existing any) bool {
	switch existing.(type) {
	case []any, []string, []bool, []int, []int64, []float64, []float32:
		return true
	}
	return false
}

func appendValue(existing any, value any) (any, error) {
	switch x := existing.(type) {
	case []any:
		return append(x, value), nil
	case []string:
		v, ok := value.(string)
		if !ok {
			return nil, appendTypeError(value, "[]string")
		}
		return append(x, v), nil
	case []bool:
		v, ok := value.(bool)
		if !ok {
			return nil, appendTypeError(value, "[]bool")
		}
		return append(x, v), nil
	case []int:
		v, ok := value.(int)
		if !ok {
			return nil, appendTypeError(value, "[]int")
		}
		return append(x, v), nil
	case []int64:
		v, ok := value.(int64)
		if !ok {
			return nil, appendTypeError(value, "[]int64")
		}
		return append(x, v), nil
	case []float64:
		v, ok := value.(float64)
		if !ok {
			return nil, appendTypeError(value, "[]float64")
		}
		return append(x, v), nil
	case []float32:
		v, ok := value.(float32)
		if !ok {
			return nil, appendTypeError(value, "[]float32")
		}
		return append(x, v), nil
	}
	return nil, fmt.Errorf("target is %T, not a supported JSON-like array", existing)
}

func appendTypeError(value any, target string) error {
	return fmt.Errorf("cannot append %T to %s", value, target)
}
