package hypertemplates

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestTemplateDataOPMLParser
// Ensure that OPML data is properly parsed into TemplateData.
func TestTemplateDataOPMLParser(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		keypaths []string
		values   []any
		err      error
	}{
		{
			name:     "resolve includes",
			source:   "testdata/templatedata/following.opml",
			keypaths: []string{"head.title", "body.outlines.length"},
			values:   []any{"Following", 195},
			err:      nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dataBytes []byte
			var data TemplateData
			var err error
			if len(tc.keypaths) != len(tc.values) {
				t.Fatalf("invalid test case with %d keypaths and %d values", len(tc.keypaths), len(tc.values))
			}
			if dataBytes, err = os.ReadFile(tc.source); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("unexpected error reading data %q bytes: %v", tc.source, err)
			}
			if data, err = ParseTemplateDataOPML(dataBytes); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("failed parsing opml %q html: %v", tc.source, err)
			}
			for i, keypath := range tc.keypaths {
				var value any = data.Get(keypath)
				if value != tc.values[i] {
					if b, _ := json.MarshalIndent(data, "", "  "); b != nil {
						t.Logf("%s\n", b)
					}
					t.Fatalf("expected keypath %q to have value %v, got %v", keypath, tc.values[i], value)
				}
			}

			// MARK: always check if we expected an error but didn't get one
			if tc.err != nil {
				t.Fatalf("expected error %q but got none", tc.err)
			}
		})
	}
}

// TestTemplateDataParser
// TODO: Ensure that Markdown, YAML, JSON, and OPML data are properly parsed into TemplateData.

// TestTemplateDataKeypathResolver
func TestTemplateDataKeypathResolver(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		keypath string
		value   any
		err     error
	}{
		{
			name:    "simple keypath",
			source:  "testdata/keypaths.yaml",
			keypath: "site.title",
			value:   "HyperTemplates Testing",
			err:     nil,
		},
		{
			name:    "nested keypath",
			source:  "testdata/keypaths.yaml",
			keypath: "data.a.b.c",
			value:   "deep value",
			err:     nil,
		},
		{
			name:    "array keypath (1-indexed, first)",
			source:  "testdata/keypaths.yaml",
			keypath: "data.users.1.name",
			value:   "Alice",
			err:     nil,
		},
		{
			name:    "array keypath (1-indexed, second)",
			source:  "testdata/keypaths.yaml",
			keypath: "data.users.2.name",
			value:   "Bob",
			err:     nil,
		},
		{
			name:    "array keypath zero segment returns nil",
			source:  "testdata/keypaths.yaml",
			keypath: "data.users.0.name",
			value:   nil,
			err:     nil,
		},
		{
			name:    "array keypath out of range returns nil",
			source:  "testdata/keypaths.yaml",
			keypath: "data.users.3.name",
			value:   nil,
			err:     nil,
		},
		{
			name:    "array keypath non-numeric segment returns nil",
			source:  "testdata/keypaths.yaml",
			keypath: "data.users.foo.name",
			value:   nil,
			err:     nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dataBytes []byte
			var data TemplateData
			var value any
			var err error
			if dataBytes, err = os.ReadFile(tc.source); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("unexpected error reading data %q bytes: %v", tc.source, err)
			}
			if data, err = ParseTemplateDataBytes(dataBytes, filepath.Ext(tc.source), nil); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("failed parsing data %q: %v", tc.source, err)
			}
			if value = data.Get(tc.keypath); value != tc.value {
				if b, _ := json.MarshalIndent(data, "", "  "); b != nil {
					t.Logf("%s\n", b)
				}
				t.Fatalf("keypath %q get\nwanted: %v\n   got: %v", tc.keypath, tc.value, value)
			}

			// MARK: always check if we expected an error but didn't get one
			if tc.err != nil && err == nil {
				t.Fatalf("expected error %q but got none", tc.err)
			}
		})
	}
}

// TestTemplateDataExistsResolver
// TODO: Add TemplateData.Exists(path string) bool method
// TODO: Ensure that TemplateData.IsEmpty() properly resolves to true when the data is empty.
// Example: { some: { key: { path: { x: null, y: [], z: false } } } }
// Results: some, some.key, and some.key.path are not empty because they are maps with at least one key.
//
//	some.key.path.x and some.key.path.y are empty because x is null and y is an empty array.
//	some.key.path.z is non-empty (bools are never "empty" values).
func TestTemplateDataExistsResolver(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		keypath string
		exists  bool
		err     error
	}{
		{
			name:    "simple key",
			source:  "testdata/exists.yaml",
			keypath: "title",
			exists:  true,
			err:     nil,
		},
		{
			name:    "simple keypath",
			source:  "testdata/exists.yaml",
			keypath: "some.value",
			exists:  true,
			err:     nil,
		},
		{
			name:    "nested keypath",
			source:  "testdata/exists.yaml",
			keypath: "some.nested.key",
			exists:  true,
			err:     nil,
		},
		{
			name:    "empty map",
			source:  "testdata/exists.yaml",
			keypath: "path.to.empty.map",
			exists:  false,
			err:     nil,
		},
		{
			name:    "map with null values",
			source:  "testdata/exists.yaml",
			keypath: "some.map.containing.keys_with_null_values",
			exists:  true,
			err:     nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dataBytes []byte
			var data TemplateData
			var exists bool
			var err error
			if dataBytes, err = os.ReadFile(tc.source); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("unexpected error reading data %q bytes: %v", tc.source, err)
			}
			if data, err = ParseTemplateDataBytes(dataBytes, filepath.Ext(tc.source), nil); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("failed parsing data %q: %v", tc.source, err)
			}

			if exists = data.Exists(tc.keypath); exists != tc.exists {
				if b, _ := json.MarshalIndent(data, "", "  "); b != nil {
					t.Logf("%s\n", b)
				}
				t.Fatalf("keypath %q exists?\nwanted: %v\n   got: %v", tc.keypath, tc.exists, exists)
			}

			// MARK: always check if we expected an error but didn't get one
			if tc.err != nil && err == nil {
				t.Fatalf("expected error %q but got none", tc.err)
			}
		})
	}
}

func TestTemplateDataClone(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var data TemplateData
		if clone := data.Clone(); clone != nil {
			t.Fatalf("expected nil clone for nil receiver, got %v", clone)
		}
	})

	t.Run("empty map returns non-nil empty map", func(t *testing.T) {
		data := TemplateData{}
		clone := data.Clone()
		if clone == nil {
			t.Fatalf("expected non-nil clone for empty map")
		}
		if len(clone) != 0 {
			t.Fatalf("expected empty clone, got %v", clone)
		}
	})

	t.Run("nested TemplateData is independent", func(t *testing.T) {
		data := TemplateData{
			"a": TemplateData{"b": "original"},
		}
		clone := data.Clone()
		clone["a"].(TemplateData)["b"] = "mutated"
		if got := data.Get("a.b"); got != "original" {
			t.Fatalf("original mutated: expected %q, got %v", "original", got)
		}
	})

	t.Run("nested map[string]any is independent", func(t *testing.T) {
		data := TemplateData{
			"a": map[string]any{"b": "original"},
		}
		clone := data.Clone()
		clone["a"].(map[string]any)["b"] = "mutated"
		if got := data.Get("a.b"); got != "original" {
			t.Fatalf("original mutated: expected %q, got %v", "original", got)
		}
	})

	t.Run("nested map[any]any is independent", func(t *testing.T) {
		data := TemplateData{
			"a": map[any]any{"b": "original"},
		}
		clone := data.Clone()
		clone["a"].(map[any]any)["b"] = "mutated"
		if got := data["a"].(map[any]any)["b"]; got != "original" {
			t.Fatalf("original mutated: expected %q, got %v", "original", got)
		}
	})

	t.Run("nested []any is independent", func(t *testing.T) {
		data := TemplateData{
			"items": []any{"x", "y"},
		}
		clone := data.Clone()
		clone["items"] = append(clone["items"].([]any), "z")
		clone["items"].([]any)[0] = "mutated"
		orig := data["items"].([]any)
		if len(orig) != 2 {
			t.Fatalf("original slice length changed: got %d, expected 2", len(orig))
		}
		if orig[0] != "x" {
			t.Fatalf("original slice element mutated: got %v, expected %q", orig[0], "x")
		}
	})

	t.Run("nested []string is independent", func(t *testing.T) {
		data := TemplateData{
			"tags": []string{"a", "b"},
		}
		clone := data.Clone()
		clone["tags"].([]string)[0] = "mutated"
		if data["tags"].([]string)[0] != "a" {
			t.Fatalf("original []string mutated")
		}
	})

	t.Run("clone serializes to same JSON as original", func(t *testing.T) {
		data := TemplateData{
			"title": "hello",
			"nested": TemplateData{
				"n":    42,
				"list": []any{"a", "b", "c"},
			},
			"tags": []string{"x", "y"},
		}
		clone := data.Clone()
		orig, err := json.Marshal(data)
		if err != nil {
			t.Fatalf("failed to marshal original: %v", err)
		}
		cloned, err := json.Marshal(clone)
		if err != nil {
			t.Fatalf("failed to marshal clone: %v", err)
		}
		if string(orig) != string(cloned) {
			t.Fatalf("clone JSON differs from original\n original: %s\n   cloned: %s", orig, cloned)
		}
	})
}

func TestTemplateDataSet(t *testing.T) {
	tests := []struct {
		name  string
		data  TemplateData
		key   string
		value any
	}{
		{
			name:  "set simple key",
			data:  TemplateData{},
			key:   "foo",
			value: "bar",
		},
		{
			name:  "set nested keypath",
			data:  TemplateData{"some": map[string]any{"key": "old value", "other": "value"}},
			key:   "some",
			value: "value",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			tc.data.Set(tc.key, tc.value)
			if value := tc.data.Get(tc.key); value != tc.value {
				t.Fatalf("failed to set key value\n\texpected %q = %v\n\t     got %q = %v", tc.key, tc.value, tc.key, value)
			}

			if jsonb, err := json.MarshalIndent(tc.data, "", "  "); err == nil {
				t.Logf("\n%s\n", jsonb)
			}
		})
	}
}

func TestTemplateDataSetKeyPath(t *testing.T) {
	tests := []struct {
		name  string
		data  TemplateData
		key   string
		value any
	}{
		{
			name:  "set simple key",
			data:  TemplateData{},
			key:   "foo",
			value: "bar",
		},
		{
			name:  "set nested keypath",
			data:  TemplateData{"some": map[string]any{"key": "value", "other": "value", "nested": map[string]any{"values": []int{0, 1, 2}}}},
			key:   "some.key",
			value: "new value",
		},
		{
			name:  "new key paths",
			data:  TemplateData{},
			key:   "some.key.nested.value",
			value: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			tc.data.SetKeyValue(tc.key, tc.value)
			if value := tc.data.Get(tc.key); value != tc.value {
				t.Fatalf("expected key %q to have value %v, got %v", tc.key, tc.value, value)
			}

			if jsonb, err := json.MarshalIndent(tc.data, "", "  "); err == nil {
				t.Logf("\n%s\n", jsonb)
			}
		})
	}
}
