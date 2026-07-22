package variable

import (
	"reflect"
	"testing"
)

type recordingGetter struct {
	values map[string]any
	calls  []string
}

func (getter *recordingGetter) Get(keypath string) any {
	getter.calls = append(getter.calls, keypath)
	return getter.values[keypath]
}

func TestResolveValuePreservesTypedValues(t *testing.T) {
	values := map[string]any{
		"value.map":          map[string]any{"key": "value"},
		"value.slice":        []string{"one", "two"},
		"value.array":        [2]int{1, 2},
		"value.string":       "text",
		"value.bool":         false,
		"value.int":          0,
		"value.empty_string": "",
		"value.empty_map":    map[string]any{},
		"value.empty_slice":  []any{},
		"value.empty_array":  [0]string{},
		"value.length()":     2,
	}

	for keypath, want := range values {
		t.Run(keypath, func(t *testing.T) {
			getter := &recordingGetter{values: values}
			got, err := ResolveValue("${ "+keypath+" }", getter)
			if err != nil {
				t.Fatalf("ResolveValue() error = %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ResolveValue() = %#v (%T), want %#v (%T)", got, got, want, want)
			}
			if !reflect.DeepEqual(getter.calls, []string{keypath}) {
				t.Fatalf("Get calls = %v, want [%s]", getter.calls, keypath)
			}
		})
	}
}

func TestResolveValueUsesOrderedNonNilFallback(t *testing.T) {
	var nilSlice []string
	getter := &recordingGetter{values: map[string]any{
		"first":  nil,
		"second": nilSlice,
		"third":  false,
		"fourth": "unreached",
	}}

	got, err := ResolveValue("${ first, second, third, fourth }", getter)
	if err != nil {
		t.Fatalf("ResolveValue() error = %v", err)
	}
	if got != false {
		t.Fatalf("ResolveValue() = %#v, want false", got)
	}
	wantCalls := []string{"first", "second", "third"}
	if !reflect.DeepEqual(getter.calls, wantCalls) {
		t.Fatalf("Get calls = %v, want %v", getter.calls, wantCalls)
	}
}

func TestResolveValueMissingFallbackChainReturnsNil(t *testing.T) {
	getter := &recordingGetter{values: map[string]any{}}

	got, err := ResolveValue("${ first, second }", getter)
	if err != nil {
		t.Fatalf("ResolveValue() error = %v", err)
	}
	if got != nil {
		t.Fatalf("ResolveValue() = %#v, want nil", got)
	}
	wantCalls := []string{"first", "second"}
	if !reflect.DeepEqual(getter.calls, wantCalls) {
		t.Fatalf("Get calls = %v, want %v", getter.calls, wantCalls)
	}
}

func TestResolveValueRejectsInvalidShapesWithoutLookup(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "naked keypath", source: "page.title"},
		{name: "leading literal text", source: "prefix ${ page.title }"},
		{name: "trailing literal text", source: "${ page.title } suffix"},
		{name: "multiple variables", source: "${ page.title }${ site.title }"},
		{name: "empty variable", source: "${ }"},
		{name: "trim markers", source: "${- page.title -}"},
		{name: "string literal", source: `${ "literal" }`},
		{name: "integer literal", source: "${ 42 }"},
		{name: "boolean literal", source: "${ false }"},
		{name: "raw get command", source: `${ get "page.title" }`},
		{name: "literal get argument", source: `${ get("page.title") }`},
		{name: "parenthetical expression", source: `${ (page.title) }`},
		{name: "function", source: `${ printf "%s" page.title }`},
		{name: "pipeline", source: "${ page.title | upper }"},
		{name: "literal fallback", source: `${ page.title, "Untitled" }`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			getter := &recordingGetter{values: map[string]any{"page.title": "title"}}
			if got, err := ResolveValue(tc.source, getter); err == nil {
				t.Fatalf("ResolveValue(%q) = %#v, nil; want syntax error", tc.source, got)
			}
			if len(getter.calls) != 0 {
				t.Fatalf("Get calls = %v, want none", getter.calls)
			}
		})
	}
}

func TestResolveValueExplicitVariablePerformsLookup(t *testing.T) {
	getter := &recordingGetter{values: map[string]any{"page.title": "title"}}

	got, err := ResolveValue("${ page.title }", getter)
	if err != nil {
		t.Fatalf("ResolveValue() error = %v", err)
	}
	if got != "title" {
		t.Fatalf("ResolveValue() = %#v, want title", got)
	}
	if !reflect.DeepEqual(getter.calls, []string{"page.title"}) {
		t.Fatalf("Get calls = %v, want [page.title]", getter.calls)
	}
}

func TestResolveValueExplicitGetPerformsOrderedLookup(t *testing.T) {
	getter := &recordingGetter{values: map[string]any{"page.title": "title"}}

	got, err := ResolveValue(`${ get(site.title, page.title) }`, getter)
	if err != nil {
		t.Fatalf("ResolveValue() error = %v", err)
	}
	if got != "title" {
		t.Fatalf("ResolveValue() = %#v, want title", got)
	}
	wantCalls := []string{"site.title", "page.title"}
	if !reflect.DeepEqual(getter.calls, wantCalls) {
		t.Fatalf("Get calls = %v, want %v", getter.calls, wantCalls)
	}
}
