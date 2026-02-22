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
// TODO: Does TemplateData.Get support keypaths with array indices? e.g., "some.array.0.key"
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
		// {
		// 	name:     "array keypath",
		// 	source:   "testdata/keypaths.yaml",
		// 	keypath:  "users.0.name",
		// 	value:    "Alice",
		// 	err:      nil,
		// },
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
