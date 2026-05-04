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
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"top-level present", "title", true},
		{"present number", "some.value", true},
		{"present nested string", "some.nested.key", true},
		{"present nested map", "some.map.containing.keys_with_null_values", true},
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
