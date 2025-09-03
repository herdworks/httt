package hypertemplates

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestTemplateData_Validate tests the validation functionality
func TestTemplateData_Validate(t *testing.T) {
	tests := []struct {
		name     string
		data     TemplateData
		required []string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid data with all required fields",
			data:     TemplateData{"title": "Test", "author": "John"},
			required: []string{"title", "author"},
			wantErr:  false,
		},
		{
			name:     "missing required field",
			data:     TemplateData{"title": "Test"},
			required: []string{"title", "author"},
			wantErr:  true,
			errMsg:   "data validation error: author is required",
		},
		{
			name:     "empty data with requirements",
			data:     TemplateData{},
			required: []string{"title"},
			wantErr:  true,
			errMsg:   "data validation error: title is required",
		},
		{
			name:     "no requirements",
			data:     TemplateData{"title": "Test"},
			required: []string{},
			wantErr:  false,
		},
		{
			name:     "nil value for required field",
			data:     TemplateData{"title": nil},
			required: []string{"title"},
			wantErr:  true,
			errMsg:   "data validation error: title is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Validate(tt.required)
			if (err != nil) != tt.wantErr {
				t.Errorf("TemplateData.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("TemplateData.Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestTemplateData_IsEmpty tests the IsEmpty method
func TestTemplateData_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		data TemplateData
		want bool
	}{
		{
			name: "empty data",
			data: TemplateData{},
			want: true,
		},
		{
			name: "nil data",
			data: nil,
			want: true,
		},
		{
			name: "data with one field",
			data: TemplateData{"title": "Test"},
			want: false,
		},
		{
			name: "data with multiple fields",
			data: TemplateData{"title": "Test", "author": "John"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.IsEmpty(); got != tt.want {
				t.Errorf("TemplateData.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTemplateData_Keys tests the Keys method
func TestTemplateData_Keys(t *testing.T) {
	tests := []struct {
		name string
		data TemplateData
		want []string
	}{
		{
			name: "empty data",
			data: TemplateData{},
			want: []string{},
		},
		{
			name: "single key",
			data: TemplateData{"title": "Test"},
			want: []string{"title"},
		},
		{
			name: "multiple keys",
			data: TemplateData{"title": "Test", "author": "John", "tags": []string{"go", "test"}},
			want: []string{"title", "author", "tags"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.data.Keys()
			if len(got) != len(tt.want) {
				t.Errorf("TemplateData.Keys() = %v, want %v", got, tt.want)
				return
			}
			// Since map iteration order is not guaranteed, check if all keys are present
			for _, key := range tt.want {
				found := false
				for _, gotKey := range got {
					if gotKey == key {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("TemplateData.Keys() missing key %v, got %v", key, got)
				}
			}
		})
	}
}

// TestTemplateData_Namespace tests the Namespace method
func TestTemplateData_Namespace(t *testing.T) {
	tests := []struct {
		name string
		data TemplateData
		want string
	}{
		{
			name: "explicit namespace",
			data: TemplateData{"namespace": "custom", "title": "Test"},
			want: "custom",
		},
		{
			name: "single key without namespace",
			data: TemplateData{"posts": []string{"post1", "post2"}},
			want: "posts",
		},
		{
			name: "multiple keys without namespace",
			data: TemplateData{"title": "Test", "author": "John"},
			want: "default",
		},
		{
			name: "empty data",
			data: TemplateData{},
			want: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.data.Namespace(); got != tt.want {
				t.Errorf("TemplateData.Namespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTemplateData_Get tests the Get method with various path structures
func TestTemplateData_Get(t *testing.T) {
	data := TemplateData{
		"title": "Test Article",
		"author": TemplateData{
			"name":  "John Doe",
			"email": "john@example.com",
		},
		"tags": []string{"go", "testing", "api"},
		"meta": TemplateData{
			"count": 42,
			"settings": TemplateData{
				"debug": true,
			},
		},
	}

	tests := []struct {
		name string
		key  string
		want any
	}{
		{
			name: "simple key",
			key:  "title",
			want: "Test Article",
		},
		{
			name: "nested key",
			key:  "author.name",
			want: "John Doe",
		},
		{
			name: "deep nested key",
			key:  "meta.settings.debug",
			want: true,
		},
		{
			name: "get keys of nested object",
			key:  "author.keys",
			want: []string{"name", "email"},
		},
		{
			name: "get length of array",
			key:  "tags.length",
			want: 3,
		},
		{
			name: "get length of object",
			key:  "author.length",
			want: 2,
		},
		{
			name: "non-existent key",
			key:  "nonexistent",
			want: nil,
		},
		{
			name: "non-existent nested key",
			key:  "author.nonexistent",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := data.Get(tt.key)
			if tt.key == "author.keys" {
				// Special handling for keys since order is not guaranteed
				gotKeys, ok := got.([]string)
				wantKeys, wantOk := tt.want.([]string)
				if !ok || !wantOk {
					t.Errorf("TemplateData.Get(%v) = %v (type %T), want %v (type %T)", tt.key, got, got, tt.want, tt.want)
					return
				}
				if len(gotKeys) != len(wantKeys) {
					t.Errorf("TemplateData.Get(%v) = %v, want %v", tt.key, got, tt.want)
					return
				}
				for _, key := range wantKeys {
					found := false
					for _, gotKey := range gotKeys {
						if gotKey == key {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("TemplateData.Get(%v) missing key %v, got %v", tt.key, key, got)
					}
				}
			} else if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TemplateData.Get(%v) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// TestTemplateData_Set tests the Set method
func TestTemplateData_Set(t *testing.T) {
	tests := []struct {
		name      string
		data      TemplateData
		namespace string
		value     any
		want      TemplateData
	}{
		{
			name:      "set new value",
			data:      TemplateData{},
			namespace: "title",
			value:     "Test",
			want:      TemplateData{"title": "Test"},
		},
		{
			name:      "overwrite existing value",
			data:      TemplateData{"title": "Old"},
			namespace: "title",
			value:     "New",
			want:      TemplateData{"title": "New"},
		},
		{
			name:      "set complex value",
			data:      TemplateData{},
			namespace: "author",
			value:     TemplateData{"name": "John", "email": "john@example.com"},
			want:      TemplateData{"author": TemplateData{"name": "John", "email": "john@example.com"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.data.Set(tt.namespace, tt.value)
			if !reflect.DeepEqual(tt.data, tt.want) {
				t.Errorf("TemplateData.Set() result = %v, want %v", tt.data, tt.want)
			}
		})
	}
}

// TestTemplateData_SetKeyValue tests the SetKeyValue method
func TestTemplateData_SetKeyValue(t *testing.T) {
	tests := []struct {
		name    string
		data    TemplateData
		key     string
		value   any
		want    TemplateData
		wantErr bool
	}{
		{
			name:    "set simple key",
			data:    TemplateData{},
			key:     "title",
			value:   "Test",
			want:    TemplateData{"title": "Test"},
			wantErr: false,
		},
		{
			name:    "set nested key on empty data",
			data:    TemplateData{},
			key:     "author.name",
			value:   "John",
			want:    TemplateData{"author": TemplateData{"name": "John"}},
			wantErr: false,
		},
		{
			name:    "set nested key on existing nested data",
			data:    TemplateData{"author": TemplateData{"email": "john@example.com"}},
			key:     "author.name",
			value:   "John",
			want:    TemplateData{"author": TemplateData{"email": "john@example.com", "name": "John"}},
			wantErr: false,
		},
		{
			name:    "overwrite simple value",
			data:    TemplateData{"title": "Old"},
			key:     "title",
			value:   "New",
			want:    TemplateData{"title": "New"},
			wantErr: false,
		},
		{
			name:    "attempt to set nested key on non-map value",
			data:    TemplateData{"title": "Test"},
			key:     "title.subtitle",
			value:   "Subtitle",
			want:    TemplateData{"title": "Test"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.SetKeyValue(tt.key, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("TemplateData.SetKeyValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(tt.data, tt.want) {
				t.Errorf("TemplateData.SetKeyValue() result = %v, want %v", tt.data, tt.want)
			}
		})
	}
}

// TestTemplateData_Delete tests the Delete method
func TestTemplateData_Delete(t *testing.T) {
	tests := []struct {
		name string
		data TemplateData
		key  string
		want TemplateData
	}{
		{
			name: "delete existing key",
			data: TemplateData{"title": "Test", "author": "John"},
			key:  "title",
			want: TemplateData{"author": "John"},
		},
		{
			name: "delete non-existing key",
			data: TemplateData{"title": "Test"},
			key:  "nonexistent",
			want: TemplateData{"title": "Test"},
		},
		{
			name: "delete from empty data",
			data: TemplateData{},
			key:  "title",
			want: TemplateData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.data.Delete(tt.key)
			if !reflect.DeepEqual(tt.data, tt.want) {
				t.Errorf("TemplateData.Delete() result = %v, want %v", tt.data, tt.want)
			}
		})
	}
}

// TestTemplateData_Merge tests the Merge functionality
func TestTemplateData_Merge(t *testing.T) {
	tests := []struct {
		name       string
		data       TemplateData
		incoming   TemplateData
		appendMode bool
		want       TemplateData
	}{
		{
			name:     "merge into empty data",
			data:     TemplateData{},
			incoming: TemplateData{"title": "Test"},
			want:     TemplateData{"title": "Test"},
		},
		{
			name:     "merge non-conflicting data",
			data:     TemplateData{"title": "Test"},
			incoming: TemplateData{"author": "John"},
			want:     TemplateData{"title": "Test", "author": "John"},
		},
		{
			name:     "merge with override",
			data:     TemplateData{"title": "Old"},
			incoming: TemplateData{"title": "New"},
			want:     TemplateData{"title": "New"},
		},
		{
			name:       "merge arrays in append mode",
			data:       TemplateData{"tags": []string{"go", "test"}},
			incoming:   TemplateData{"tags": []string{"api", "web"}},
			appendMode: true,
			want:       TemplateData{"tags": []string{"go", "test", "api", "web"}},
		},
		{
			name:       "merge arrays in replace mode",
			data:       TemplateData{"tags": []string{"go", "test"}},
			incoming:   TemplateData{"tags": []string{"api", "web"}},
			appendMode: false,
			want:       TemplateData{"tags": []string{"api", "web"}},
		},
		{
			name: "merge nested TemplateData",
			data: TemplateData{"author": TemplateData{"name": "John"}},
			incoming: TemplateData{"author": TemplateData{"email": "john@example.com"}},
			want: TemplateData{"author": TemplateData{"name": "John", "email": "john@example.com"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.data.Merge(tt.incoming, tt.appendMode, false)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TemplateData.Merge() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTemplateData_Clean tests the Clean method
func TestTemplateData_Clean(t *testing.T) {
	tests := []struct {
		name     string
		data     TemplateData
		reserved []string
		want     TemplateData
	}{
		{
			name:     "clean with no reserved keys",
			data:     TemplateData{"title": "Test", "author": "John"},
			reserved: []string{},
			want:     TemplateData{"title": "Test", "author": "John"},
		},
		{
			name:     "clean with reserved keys",
			data:     TemplateData{"title": "Test", "layout": "main", "author": "John"},
			reserved: []string{"layout"},
			want:     TemplateData{"title": "Test", "author": "John"},
		},
		{
			name:     "clean with multiple reserved keys",
			data:     TemplateData{"title": "Test", "layout": "main", "path": "/test", "author": "John"},
			reserved: []string{"layout", "path"},
			want:     TemplateData{"title": "Test", "author": "John"},
		},
		{
			name:     "clean with all keys reserved",
			data:     TemplateData{"layout": "main", "path": "/test"},
			reserved: []string{"layout", "path"},
			want:     TemplateData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.data.Clean(tt.reserved)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TemplateData.Clean() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTemplateData_Inspect tests serialization to different formats
func TestTemplateData_Inspect(t *testing.T) {
	data := TemplateData{
		"title": "Test",
		"author": TemplateData{
			"name":  "John",
			"email": "john@example.com",
		},
		"tags": []string{"go", "test"},
	}

	tests := []struct {
		name       string
		path       []string
		format     string
		wantErr    bool
		checkFunc  func([]byte) bool
	}{
		{
			name:   "inspect as JSON",
			path:   []string{},
			format: "json",
			wantErr: false,
			checkFunc: func(b []byte) bool {
				var result map[string]any
				return json.Unmarshal(b, &result) == nil
			},
		},
		{
			name:   "inspect as YAML",
			path:   []string{},
			format: "yaml",
			wantErr: false,
			checkFunc: func(b []byte) bool {
				var result map[string]any
				return yaml.Unmarshal(b, &result) == nil
			},
		},
		{
			name:    "inspect as XML",
			path:    []string{},
			format:  "xml",
			wantErr: true, // TemplateData (map[string]any) doesn't support XML marshaling
		},
		{
			name:    "inspect nested path as JSON",
			path:    []string{"author"},
			format:  "json",
			wantErr: false,
			checkFunc: func(b []byte) bool {
				var result map[string]any
				return json.Unmarshal(b, &result) == nil && strings.Contains(string(b), "John")
			},
		},
		{
			name:    "unsupported format",
			path:    []string{},
			format:  "unsupported",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := data.Inspect(tt.path, tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("TemplateData.Inspect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFunc != nil && !tt.checkFunc(got) {
				t.Errorf("TemplateData.Inspect() result validation failed for format %s", tt.format)
			}
		})
	}
}

// TestParseTemplateDataBytes tests parsing from different data formats
func TestParseTemplateDataBytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		format   string
		want     TemplateData
		wantErr  bool
	}{
		{
			name:   "parse JSON",
			data:   []byte(`{"title": "Test", "author": "John"}`),
			format: ".json",
			want:   TemplateData{"title": "Test", "author": "John"},
			wantErr: false,
		},
		{
			name:   "parse YAML",
			data:   []byte("title: Test\nauthor: John\n"),
			format: ".yaml",
			want:   TemplateData{"title": "Test", "author": "John"},
			wantErr: false,
		},
		{
			name:   "parse YML",
			data:   []byte("title: Test\nauthor: John\n"),
			format: ".yml",
			want:   TemplateData{"title": "Test", "author": "John"},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{"title": "Test", "author": `),
			format:  ".json",
			wantErr: true,
		},
		{
			name:    "invalid YAML",
			data:    []byte("title: Test\n  author: John\n    invalid"),
			format:  ".yaml",
			wantErr: true,
		},
		{
			name:    "unsupported format",
			data:    []byte("some data"),
			format:  ".txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTemplateDataBytes(tt.data, tt.format, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTemplateDataBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTemplateDataBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTemplateData_StringifyMapKeys tests the map key stringification
func TestTemplateData_StringifyMapKeys(t *testing.T) {
	// Create data with non-string keys (simulating yaml.v2 output)
	data := TemplateData{
		"normal": "value",
		"nested": map[any]any{
			123:     "numeric key",
			"string": "string key",
			true:    "boolean key",
		},
		"array": []any{
			map[any]any{
				456:   "in array",
				false: "boolean in array",
			},
		},
	}

	data.StringifyMapKeys()

	// Check that nested map[any]any was converted to map[string]any
	if nested, ok := data["nested"].(map[string]any); ok {
		if _, ok := nested["123"]; !ok {
			t.Error("Expected numeric key to be stringified to '123'")
		}
		if _, ok := nested["true"]; !ok {
			t.Error("Expected boolean key to be stringified to 'true'")
		}
		if _, ok := nested["string"]; !ok {
			t.Error("Expected string key to remain as 'string'")
		}
	} else {
		t.Error("Expected nested map[any]any to be converted to map[string]any")
	}

	// Check array nested maps
	if array, ok := data["array"].([]any); ok {
		if len(array) > 0 {
			if nestedInArray, ok := array[0].(map[string]any); ok {
				if _, ok := nestedInArray["456"]; !ok {
					t.Error("Expected numeric key in array to be stringified to '456'")
				}
				if _, ok := nestedInArray["false"]; !ok {
					t.Error("Expected boolean key in array to be stringified to 'false'")
				}
			} else {
				t.Error("Expected nested map in array to be converted to map[string]any")
			}
		}
	}
}

// TestOrderedTemplateData_MarshalYAML tests the ordered YAML marshaling
func TestOrderedTemplateData_MarshalYAML(t *testing.T) {
	data := OrderedTemplateData{
		"title":       "Test Article",
		"description": "A test article",
		"author":      "John Doe",
		"custom":      "custom field",
		"created_at":  "2023-01-01",
		"layout":      "post",
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		t.Fatalf("OrderedTemplateData.MarshalYAML() error = %v", err)
	}

	yamlStr := string(yamlBytes)
	
	// Check that ordered fields appear first
	orderedFields := []string{"created_at", "layout", "title", "description", "author"}
	lastPos := -1
	for _, field := range orderedFields {
		pos := strings.Index(yamlStr, field+":")
		if pos == -1 {
			continue // field not present, skip
		}
		if pos < lastPos {
			t.Errorf("Field %s appears before a previously checked field, YAML is not properly ordered", field)
		}
		lastPos = pos
	}

	// Ensure custom fields appear after ordered fields
	customPos := strings.Index(yamlStr, "custom:")
	if customPos != -1 && customPos < lastPos {
		t.Error("Custom field appears before ordered fields")
	}
}

// Benchmark tests for performance-critical methods
func BenchmarkTemplateData_Get(b *testing.B) {
	data := TemplateData{
		"level1": TemplateData{
			"level2": TemplateData{
				"level3": "deep_value",
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.Get("level1.level2.level3")
	}
}

func BenchmarkTemplateData_Merge(b *testing.B) {
	data1 := TemplateData{
		"title":  "Test",
		"author": "John",
		"tags":   []string{"go", "test"},
	}
	data2 := TemplateData{
		"description": "A test article",
		"category":    "testing",
		"tags":        []string{"api", "web"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data1.Merge(data2, false, false)
	}
}

func BenchmarkTemplateData_SetKeyValue(b *testing.B) {
	data := TemplateData{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data.SetKeyValue("level1.level2.level3", "value")
	}
}