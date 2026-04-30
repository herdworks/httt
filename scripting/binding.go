package scripting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dop251/goja"
)

type bindingMode uint8

const (
	bindingModeInput bindingMode = iota
	bindingModeMap
)

const inputBindingCacheToken = "input"

type bindingShape struct {
	mode bindingMode
	keys []string
}

func bindingShapeForArg(arg any) bindingShape {
	data, ok := jsPlainMap(arg)
	if !ok {
		return bindingShape{mode: bindingModeInput}
	}

	return bindingShape{
		mode: bindingModeMap,
		keys: sortedKeys(data),
	}
}

func (s bindingShape) cacheKey(src string) string {
	var b strings.Builder
	b.Grow(len(src) + len(inputBindingCacheToken) + 16)
	b.WriteString(src)
	b.WriteByte(0)

	if s.mode == bindingModeInput {
		b.WriteString(inputBindingCacheToken)
		return b.String()
	}

	b.WriteString("map")
	for _, key := range s.keys {
		b.WriteByte(0)
		b.WriteString(key)
	}

	return b.String()
}

func (s bindingShape) wrapperSource(src string) string {
	var b strings.Builder
	b.Grow(len(src) + 64)
	b.WriteString(`(function(__arg){ "use strict"; `)

	switch s.mode {
	case bindingModeInput:
		b.WriteString(`const input = __arg; `)
	case bindingModeMap:
		if len(s.keys) > 0 {
			b.WriteString(`const {`)
			b.WriteString(strings.Join(s.keys, ","))
			b.WriteString(`} = __arg; `)
		}
	}

	b.WriteString(src)
	b.WriteString(` })`)
	return b.String()
}

func (s bindingShape) validateCallArg(arg any) error {
	switch s.mode {
	case bindingModeInput:
		if _, ok := jsPlainMap(arg); ok {
			return fmt.Errorf("scripting: expected non-map input, got %T", arg)
		}
		return nil
	case bindingModeMap:
		data, ok := jsPlainMap(arg)
		if !ok {
			return fmt.Errorf("scripting: expected map[string]any input, got %T", arg)
		}

		missing := missingKeys(data, s.keys)
		if len(missing) == 0 {
			return nil
		}

		return fmt.Errorf("scripting: missing required top-level keys: %s", strings.Join(missing, ", "))
	default:
		return fmt.Errorf("scripting: unsupported binding mode %d", s.mode)
	}
}

func (s bindingShape) validateLocalBindings() error {
	if s.mode != bindingModeMap || len(s.keys) == 0 {
		return nil
	}

	invalid := invalidBindingKeys(s.keys)
	if len(invalid) == 0 {
		return nil
	}

	return fmt.Errorf(
		"scripting: top-level map keys must be valid strict-mode JavaScript binding names: %s",
		quotedKeys(invalid),
	)
}

func sortedKeys(data map[string]any) []string {
	if len(data) == 0 {
		return nil
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

func missingKeys(data map[string]any, keys []string) []string {
	var missing []string
	for _, key := range keys {
		if _, ok := data[key]; ok {
			continue
		}
		missing = append(missing, key)
	}
	return missing
}

func invalidBindingKeys(keys []string) []string {
	var invalid []string
	for _, key := range keys {
		if bindingKeyIsValid(key) {
			continue
		}
		invalid = append(invalid, key)
	}
	return invalid
}

func bindingKeyIsValid(key string) bool {
	src := `(function(__arg){ "use strict"; const {` + key + `} = __arg; return undefined; })`
	_, err := goja.Compile("[binding.go]", src, false)
	return err == nil
}

func quotedKeys(keys []string) string {
	quoted := make([]string, 0, len(keys))
	for _, key := range keys {
		quoted = append(quoted, fmt.Sprintf("%q", key))
	}

	return strings.Join(quoted, ", ")
}
