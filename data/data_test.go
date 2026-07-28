package data

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

const keypathsYAML = `
site:
  title: HyperTemplates Testing
  description: A site for testing HyperTemplates features and functionalities.

data:
  a:
    b:
      c: deep value
  users:
    - name: Alice
      role: admin
    - name: Bob
      role: user
`

const existsYAML = `
title: Testing Has Keypaths
some:
  value: 42
  nested:
    key: nested value
  map:
    containing:
      keys_with_null_values:
        a: null
path:
  to:
    empty:
      map: {}
      array: []
      slice: []
`

func mustYAML(t *testing.T, src string) TemplateData {
	t.Helper()
	d, err := UnmarshalYAML([]byte(src))
	if err != nil {
		t.Fatalf("yaml unmarshal failed: %v", err)
	}
	return d
}

func TestGet(t *testing.T) {
	d := mustYAML(t, keypathsYAML)
	tests := []struct {
		name string
		key  string
		want any
	}{
		{"simple keypath", "site.title", "HyperTemplates Testing"},
		{"nested keypath", "data.a.b.c", "deep value"},
		{"array first", "data.users.1.name", "Alice"},
		{"array second", "data.users.2.name", "Bob"},
		{"array zero is nil", "data.users.0.name", nil},
		{"array out of range", "data.users.3.name", nil},
		{"array non-numeric segment", "data.users.foo.name", nil},
		{"missing top-level", "nope", nil},
		{"missing nested", "site.nope.deeper", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.Get(tc.key)
			if got != tc.want {
				if b, _ := json.MarshalIndent(d, "", "  "); b != nil {
					t.Logf("data:\n%s", b)
				}
				t.Fatalf("Get(%q)\nwant: %v\n got: %v", tc.key, tc.want, got)
			}
		})
	}
}

func TestGetEmptyKeyReturnsReceiver(t *testing.T) {
	d := TemplateData{"a": 1}
	got := d.Get("")
	gd, ok := got.(TemplateData)
	if !ok {
		t.Fatalf("Get(\"\") returned %T, want TemplateData", got)
	}
	if gd["a"] != 1 {
		t.Fatalf("Get(\"\") returned %v, want %v", got, d)
	}
}

func TestHas(t *testing.T) {
	d := mustYAML(t, existsYAML)
	d["truthiness"] = TemplateData{
		"false": false,
		"true":  true,
		"zero":  0,
	}
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"top-level present", "title", true},
		{"present number", "some.value", true},
		{"present nested string", "some.nested.key", true},
		{"present nested map", "some.map.containing.keys_with_null_values", true},
		{"boolean false", "truthiness.false", false},
		{"boolean true", "truthiness.true", true},
		{"numeric zero", "truthiness.zero", true},
		{"empty map", "path.to.empty.map", false},
		{"empty array", "path.to.empty.array", false},
		{"missing key", "nope", false},
		{"present null", "some.map.containing.keys_with_null_values.a", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := d.Has(tc.key); got != tc.want {
				t.Fatalf("Has(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}

func TestHasTruthiness(t *testing.T) {
	var nilSlice []string
	var nilMap map[string]any
	var nilPointer *int
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "nil", value: nil, want: false},
		{name: "typed nil slice", value: nilSlice, want: false},
		{name: "typed nil map", value: nilMap, want: false},
		{name: "typed nil pointer", value: nilPointer, want: false},
		{name: "false", value: false, want: false},
		{name: "true", value: true, want: true},
		{name: "integer zero", value: 0, want: true},
		{name: "floating point zero", value: 0.0, want: true},
		{name: "empty string", value: "", want: false},
		{name: "non-empty string", value: "value", want: true},
		{name: "empty slice", value: []string{}, want: false},
		{name: "non-empty slice", value: []string{"value"}, want: true},
		{name: "empty array", value: [0]string{}, want: false},
		{name: "non-empty array", value: [1]string{"value"}, want: true},
		{name: "empty map", value: map[string]any{}, want: false},
		{name: "non-empty map", value: map[string]any{"key": nil}, want: true},
		{name: "other non-nil value", value: struct{}{}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := TemplateData{"value": tc.value}
			if got := data.Has("value"); got != tc.want {
				t.Fatalf("Has(value) = %v, want %v for %#v", got, tc.want, tc.value)
			}
		})
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		name  string
		data  TemplateData
		key   string
		value any
		want  any
	}{
		{
			name:  "set top-level",
			data:  TemplateData{},
			key:   "foo",
			value: "bar",
			want:  "bar",
		},
		{
			name:  "overwrite existing",
			data:  TemplateData{"foo": "old"},
			key:   "foo",
			value: "new",
			want:  "new",
		},
		{
			name:  "create nested path from empty",
			data:  TemplateData{},
			key:   "a.b.c.d",
			value: 42,
			want:  42,
		},
		{
			name:  "descend into existing nested map",
			data:  TemplateData{"some": map[string]any{"key": "old", "other": "kept"}},
			key:   "some.key",
			value: "new",
			want:  "new",
		},
		{
			name:  "descend into existing TemplateData",
			data:  TemplateData{"some": TemplateData{"key": "old"}},
			key:   "some.key",
			value: "new",
			want:  "new",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.data.Set(tc.key, tc.value); err != nil {
				t.Fatalf("Set(%q): %v", tc.key, err)
			}
			if got := tc.data.Get(tc.key); got != tc.want {
				t.Fatalf("after Set(%q, %v): Get returned %v, want %v", tc.key, tc.value, got, tc.want)
			}
		})
	}
}

func TestSetBlockedByScalar(t *testing.T) {
	d := TemplateData{"a": "scalar"}
	err := d.Set("a.b", 1)
	if err == nil {
		t.Fatalf("expected error setting a.b when a is a scalar; got nil")
	}
	if got := d.Get("a"); got != "scalar" {
		t.Fatalf("Set should not mutate blocked path; got a=%v", got)
	}
}

func TestSetEmptyKeyErrors(t *testing.T) {
	d := TemplateData{}
	if err := d.Set("", "x"); err == nil {
		t.Fatalf("expected error on empty key")
	}
}

func TestTemplateDataKeyPathParse(t *testing.T) {
	var keyPath TemplateDataKeyPath
	if err := keyPath.parse("page.author.name"); err != nil {
		t.Fatalf("parse() error = %v", err)
	}
	if keyPath.Raw != "page.author.name" {
		t.Fatalf("Raw = %q, want page.author.name", keyPath.Raw)
	}
	if !slices.Equal(keyPath.Path, []string{"page", "author"}) {
		t.Fatalf("Path = %v, want [page author]", keyPath.Path)
	}
	if keyPath.Key != "name" {
		t.Fatalf("Key = %q, want name", keyPath.Key)
	}

	var topLevel TemplateDataKeyPath
	if err := topLevel.parse("title"); err != nil {
		t.Fatalf("top-level parse() error = %v", err)
	}
	if topLevel.Raw != "title" || len(topLevel.Path) != 0 || topLevel.Key != "title" {
		t.Fatalf("top-level parse = %#v, want Raw:title Path:nil Key:title", topLevel)
	}
}

func TestTemplateDataKeyPathReachability(t *testing.T) {
	d := TemplateData{
		"page": TemplateData{
			"metadata": map[string]any{},
			"tags":     []any{"go"},
		},
	}

	var existing TemplateDataKeyPath
	if err := existing.parse("page.metadata.title"); err != nil {
		t.Fatalf("parse existing path: %v", err)
	}
	if err := existing.isReachable(d); err != nil {
		t.Fatalf("existing path should be reachable: %v", err)
	}
	if !existing.branchExists(d) {
		t.Fatalf("existing branch path should exist")
	}

	var missing TemplateDataKeyPath
	if err := missing.parse("page.details.title"); err != nil {
		t.Fatalf("parse missing path: %v", err)
	}
	if err := missing.isReachable(d); err != nil {
		t.Fatalf("missing branch path should be reachable: %v", err)
	}
	if missing.branchExists(d) {
		t.Fatalf("missing branch path should not exist")
	}

	var blocked TemplateDataKeyPath
	if err := blocked.parse("page.tags.slug"); err != nil {
		t.Fatalf("parse blocked path: %v", err)
	}
	if err := blocked.isReachable(d); err == nil {
		t.Fatalf("array branch path should not be reachable")
	}
}

func TestAdd(t *testing.T) {
	d := TemplateData{}
	if err := d.Add("a.b", "new"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if got := d.Get("a.b"); got != "new" {
		t.Fatalf("a.b = %v, want new", got)
	}
	if err := d.Add("a.b", "duplicate"); err == nil {
		t.Fatalf("Add() duplicate error = nil, want error")
	}
	if got := d.Get("a.b"); got != "new" {
		t.Fatalf("duplicate Add mutated a.b = %v, want new", got)
	}
}

func TestAddBlockedByScalar(t *testing.T) {
	d := TemplateData{"a": "scalar"}
	err := d.Add("a.b", 1)
	if err == nil {
		t.Fatalf("Add() error = nil, want scalar path error")
	}
	if got := d.Get("a"); got != "scalar" {
		t.Fatalf("Add should not mutate blocked path; got a=%v", got)
	}
}

func TestReplace(t *testing.T) {
	d := TemplateData{"a": TemplateData{"b": "old"}}
	if err := d.Replace("a.b", "new"); err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if got := d.Get("a.b"); got != "new" {
		t.Fatalf("a.b = %v, want new", got)
	}
	if err := d.Replace("a.c", "missing"); err == nil {
		t.Fatalf("Replace() missing leaf error = nil, want error")
	}
	if got := d.Get("a.c"); got != nil {
		t.Fatalf("Replace created missing a.c = %v", got)
	}
}

func TestReplaceBlockedByMissingParentAndScalar(t *testing.T) {
	missing := TemplateData{}
	if err := missing.Replace("a.b", 1); err == nil {
		t.Fatalf("Replace() missing parent error = nil, want error")
	}
	scalar := TemplateData{"a": "scalar"}
	if err := scalar.Replace("a.b", 1); err == nil {
		t.Fatalf("Replace() scalar parent error = nil, want error")
	}
}

func TestAppend(t *testing.T) {
	d := TemplateData{
		"tags":    []string{"existing"},
		"items":   []any{1},
		"flags":   []bool{true},
		"numbers": []int{1},
		"large":   []int64{1},
		"ratios":  []float64{1.5},
		"small":   []float32{1.5},
	}
	if err := d.Append("tags", "new"); err != nil {
		t.Fatalf("Append(tags) error = %v", err)
	}
	if got := d.Get("tags"); !reflect.DeepEqual(got, []string{"existing", "new"}) {
		t.Fatalf("tags = %#v, want appended []string", got)
	}
	if err := d.Append("items", 2); err != nil {
		t.Fatalf("Append(items) error = %v", err)
	}
	if got := d.Get("items"); !reflect.DeepEqual(got, []any{1, 2}) {
		t.Fatalf("items = %#v, want appended []any", got)
	}
	if err := d.Append("flags", false); err != nil {
		t.Fatalf("Append(flags) error = %v", err)
	}
	if got := d.Get("flags"); !reflect.DeepEqual(got, []bool{true, false}) {
		t.Fatalf("flags = %#v, want appended []bool", got)
	}
	if err := d.Append("numbers", 2); err != nil {
		t.Fatalf("Append(numbers) error = %v", err)
	}
	if got := d.Get("numbers"); !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("numbers = %#v, want appended []int", got)
	}
	if err := d.Append("large", int64(2)); err != nil {
		t.Fatalf("Append(large) error = %v", err)
	}
	if got := d.Get("large"); !reflect.DeepEqual(got, []int64{1, 2}) {
		t.Fatalf("large = %#v, want appended []int64", got)
	}
	if err := d.Append("ratios", 2.5); err != nil {
		t.Fatalf("Append(ratios) error = %v", err)
	}
	if got := d.Get("ratios"); !reflect.DeepEqual(got, []float64{1.5, 2.5}) {
		t.Fatalf("ratios = %#v, want appended []float64", got)
	}
	if err := d.Append("small", float32(2.5)); err != nil {
		t.Fatalf("Append(small) error = %v", err)
	}
	if got := d.Get("small"); !reflect.DeepEqual(got, []float32{1.5, 2.5}) {
		t.Fatalf("small = %#v, want appended []float32", got)
	}
	if err := d.Append("categories", "created"); err != nil {
		t.Fatalf("Append(categories) error = %v", err)
	}
	if got := d.Get("categories"); !reflect.DeepEqual(got, []any{"created"}) {
		t.Fatalf("categories = %#v, want created []any", got)
	}
	if err := d.Append("nested.values", true); err != nil {
		t.Fatalf("Append(nested.values) error = %v", err)
	}
	if got := d.Get("nested.values"); !reflect.DeepEqual(got, []any{true}) {
		t.Fatalf("nested.values = %#v, want created nested []any", got)
	}
}

func TestAppendRejectsScalarIncompatibleAndUnsupportedSlices(t *testing.T) {
	scalar := TemplateData{"tags": "not an array"}
	if err := scalar.Append("tags", "new"); err == nil {
		t.Fatalf("Append() scalar target error = nil, want error")
	}
	if got := scalar.Get("tags"); got != "not an array" {
		t.Fatalf("Append mutated scalar target = %v", got)
	}
	typed := TemplateData{"numbers": []int{1}}
	if err := typed.Append("numbers", "two"); err == nil {
		t.Fatalf("Append() incompatible typed slice error = nil, want error")
	}
	if got := typed.Get("numbers"); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("Append mutated incompatible typed slice = %#v", got)
	}
	unsupported := TemplateData{"numbers": []uint{1}}
	if err := unsupported.Append("numbers", uint(2)); err == nil {
		t.Fatalf("Append() unsupported typed slice error = nil, want error")
	}
	if got := unsupported.Get("numbers"); !reflect.DeepEqual(got, []uint{1}) {
		t.Fatalf("Append mutated unsupported typed slice = %#v", got)
	}
	array := TemplateData{"numbers": [1]int{1}}
	if err := array.Append("numbers", 2); err == nil {
		t.Fatalf("Append() array error = nil, want error")
	}
	if got := array.Get("numbers"); !reflect.DeepEqual(got, [1]int{1}) {
		t.Fatalf("Append mutated array = %#v", got)
	}
}

func TestMutationKeysRejectEmptyAndFunctionSegments(t *testing.T) {
	for _, mutate := range []struct {
		name string
		fn   func(string, any) error
	}{
		{name: "Set", fn: TemplateData{}.Set},
		{name: "Add", fn: TemplateData{}.Add},
		{name: "Replace", fn: TemplateData{"a": 1}.Replace},
		{name: "Append", fn: TemplateData{}.Append},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if err := mutate.fn("", "x"); err == nil {
				t.Fatalf("%s empty key error = nil, want error", mutate.name)
			}
			if err := mutate.fn("a..b", "x"); err == nil {
				t.Fatalf("%s empty segment error = nil, want error", mutate.name)
			}
			if err := mutate.fn("a.length()", "x"); err == nil {
				t.Fatalf("%s function segment error = nil, want error", mutate.name)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	d := TemplateData{
		"a": "x",
		"b": TemplateData{
			"c": "keep",
			"d": "drop",
		},
	}
	d.Delete("a")
	if _, ok := d["a"]; ok {
		t.Fatalf("Delete(\"a\") did not remove top-level key")
	}
	d.Delete("b.d")
	if d.Get("b.d") != nil {
		t.Fatalf("Delete(\"b.d\") did not remove nested key")
	}
	if d.Get("b.c") != "keep" {
		t.Fatalf("Delete(\"b.d\") removed sibling key b.c")
	}
	d.Delete("missing.path") // no-op, must not panic
}

func TestKeysAndLen(t *testing.T) {
	d := mustYAML(t, keypathsYAML)
	wantTop := []string{"data", "site"}
	if got := d.Keys(""); !slices.Equal(got, wantTop) {
		t.Fatalf("Keys(\"\") = %v, want %v", got, wantTop)
	}
	wantNested := []string{"description", "title"}
	if got := d.Keys("site"); !slices.Equal(got, wantNested) {
		t.Fatalf("Keys(\"site\") = %v, want %v", got, wantNested)
	}
	if got := d.Len("data.users"); got != 2 {
		t.Fatalf("Len(\"data.users\") = %d, want 2", got)
	}
	if got := d.Len("site.title"); got != len("HyperTemplates Testing") {
		t.Fatalf("Len(\"site.title\") = %d, want %d", got, len("HyperTemplates Testing"))
	}
}

func TestFunctionStyleAccessors(t *testing.T) {
	d := mustYAML(t, keypathsYAML)
	tests := []struct {
		name string
		key  string
		want any
	}{
		{"length on slice", "data.users.length()", 2},
		{"count alias on slice", "data.users.count()", 2},
		{"length on map", "site.length()", 2},
		{"length on string", "site.title.length()", len("HyperTemplates Testing")},
		{"keys on map", "site.keys()", []string{"description", "title"}},
		{"length at root", "length()", 2},
		{"unknown function returns nil", "site.nope()", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.Get(tc.key)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Get(%q)\nwant: %v\n got: %v", tc.key, tc.want, got)
			}
		})
	}
}

func TestClone(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		var d TemplateData
		if d.Clone() != nil {
			t.Fatalf("expected nil clone for nil receiver")
		}
	})
	t.Run("empty returns non-nil empty", func(t *testing.T) {
		d := TemplateData{}
		c := d.Clone()
		if c == nil || len(c) != 0 {
			t.Fatalf("expected non-nil empty clone, got %v", c)
		}
	})
	t.Run("nested TemplateData independent", func(t *testing.T) {
		d := TemplateData{"a": TemplateData{"b": "original"}}
		c := d.Clone()
		c["a"].(TemplateData)["b"] = "mutated"
		if got := d.Get("a.b"); got != "original" {
			t.Fatalf("original mutated: got %v", got)
		}
	})
	t.Run("nested map[string]any independent", func(t *testing.T) {
		d := TemplateData{"a": map[string]any{"b": "original"}}
		c := d.Clone()
		c["a"].(map[string]any)["b"] = "mutated"
		if got := d.Get("a.b"); got != "original" {
			t.Fatalf("original mutated: got %v", got)
		}
	})
	t.Run("nested []any independent", func(t *testing.T) {
		d := TemplateData{"items": []any{"x", "y"}}
		c := d.Clone()
		c["items"] = append(c["items"].([]any), "z")
		c["items"].([]any)[0] = "mutated"
		orig := d["items"].([]any)
		if len(orig) != 2 || orig[0] != "x" {
			t.Fatalf("original slice mutated: %v", orig)
		}
	})
	t.Run("nested []string independent", func(t *testing.T) {
		d := TemplateData{"tags": []string{"a", "b"}}
		c := d.Clone()
		c["tags"].([]string)[0] = "mutated"
		if d["tags"].([]string)[0] != "a" {
			t.Fatalf("original []string mutated")
		}
	})
	t.Run("clone JSON-equal to original", func(t *testing.T) {
		d := TemplateData{
			"title": "hello",
			"nested": TemplateData{
				"n":    42,
				"list": []any{"a", "b", "c"},
			},
			"tags": []string{"x", "y"},
		}
		c := d.Clone()
		want, _ := json.Marshal(d)
		got, _ := json.Marshal(c)
		if string(want) != string(got) {
			t.Fatalf("JSON differs:\n  want: %s\n   got: %s", want, got)
		}
	})
}

func TestShallowClone(t *testing.T) {
	t.Run("nil receiver and nil map are writable", func(t *testing.T) {
		var nilPointer *TemplateData
		clone := nilPointer.ShallowClone()
		clone["title"] = "fallback"
		if got := clone.Get("title"); got != "fallback" {
			t.Fatalf("nil receiver clone = %#v", got)
		}

		var nilMap TemplateData
		clone = (&nilMap).ShallowClone()
		clone["title"] = "fallback"
		if got := clone.Get("title"); got != "fallback" {
			t.Fatalf("nil map clone = %#v", got)
		}
	})

	t.Run("copies only top-level bindings", func(t *testing.T) {
		original := TemplateData{
			"title": "original",
			"page":  TemplateData{"title": "nested"},
		}
		clone := (&original).ShallowClone()
		clone["title"] = "clone"
		clone.Get("page").(TemplateData)["title"] = "shared nested"

		if got := original.Get("title"); got != "original" {
			t.Fatalf("original top-level value = %#v, want original", got)
		}
		if got := original.Get("page.title"); got != "shared nested" {
			t.Fatalf("original nested value = %#v, want shared nested", got)
		}
	})
}

func TestMerge(t *testing.T) {
	base := TemplateData{
		"a": "keep",
		"b": TemplateData{"x": 1, "y": 2},
		"c": []any{"one"},
	}
	other := TemplateData{
		"b": TemplateData{"y": 20, "z": 30},
		"c": []any{"two"},
		"d": "new",
	}
	merged := base.Merge(other)
	if merged.Get("a") != "keep" {
		t.Errorf("a: want keep, got %v", merged.Get("a"))
	}
	if merged.Get("b.x") != 1 || merged.Get("b.y") != 20 || merged.Get("b.z") != 30 {
		t.Errorf("b: want {x:1,y:20,z:30}, got %v", merged.Get("b"))
	}
	if got := merged.Get("c").([]any); !reflect.DeepEqual(got, []any{"two"}) {
		t.Errorf("c: want replace, got %v", got)
	}
	if merged.Get("d") != "new" {
		t.Errorf("d: want new, got %v", merged.Get("d"))
	}
}

func TestMergeAppend(t *testing.T) {
	base := TemplateData{
		"tags":  []string{"a", "b"},
		"items": []any{1, 2},
	}
	other := TemplateData{
		"tags":  []string{"c"},
		"items": []any{3},
	}
	merged := base.MergeAppend(other)
	if got := merged.Get("tags").([]string); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("tags: want [a b c], got %v", got)
	}
	if got := merged.Get("items").([]any); !reflect.DeepEqual(got, []any{1, 2, 3}) {
		t.Errorf("items: want [1 2 3], got %v", got)
	}
}

func TestWithout(t *testing.T) {
	d := TemplateData{"keep": 1, "drop": 2, "alsodrop": 3}
	out := d.Without("drop", "alsodrop")
	if _, ok := out["drop"]; ok {
		t.Errorf("drop not removed")
	}
	if _, ok := out["alsodrop"]; ok {
		t.Errorf("alsodrop not removed")
	}
	if out["keep"] != 1 {
		t.Errorf("keep removed or mutated")
	}
	if _, ok := d["drop"]; !ok {
		t.Errorf("Without mutated receiver")
	}
}

func TestStringifyKeys(t *testing.T) {
	d := TemplateData{
		"front": map[any]any{
			"title":  "T",
			"nested": map[any]any{"k": "v"},
		},
	}
	out := d.StringifyKeys()
	front, ok := out["front"].(map[string]any)
	if !ok {
		t.Fatalf("front not converted: %T", out["front"])
	}
	if front["title"] != "T" {
		t.Errorf("title lost: %v", front["title"])
	}
	nested, ok := front["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested not converted: %T", front["nested"])
	}
	if nested["k"] != "v" {
		t.Errorf("nested.k lost: %v", nested["k"])
	}
	if _, ok := d["front"].(map[any]any); !ok {
		t.Errorf("StringifyKeys mutated receiver")
	}
}

func TestUnmarshalJSON(t *testing.T) {
	d, err := UnmarshalJSON([]byte(`{"a":1,"b":{"c":"x"}}`))
	if err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	if got := d.Get("b.c"); got != "x" {
		t.Fatalf("b.c = %v, want x", got)
	}
}

func TestUnmarshalYAML(t *testing.T) {
	d, err := UnmarshalYAML([]byte("a: 1\nb:\n  c: x\n"))
	if err != nil {
		t.Fatalf("UnmarshalYAML: %v", err)
	}
	if got := d.Get("b.c"); got != "x" {
		t.Fatalf("b.c = %v, want x", got)
	}
}

func TestRoundTripWithMapStringAny(t *testing.T) {
	m := map[string]any{"a": 1, "b": "x"}
	d := TemplateData(m)
	if d.Get("a") != 1 || d.Get("b") != "x" {
		t.Fatalf("conversion from map[string]any failed")
	}
	back := map[string]any(d)
	if back["a"] != 1 || back["b"] != "x" {
		t.Fatalf("conversion back to map[string]any failed")
	}
}
