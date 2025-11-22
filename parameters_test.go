package hypertemplates

import (
	"reflect"
	"testing"
)

// Helper function to compare slices that might be nil vs empty
func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// TestParameter_Parse tests the parsing of parameter attribute strings
func TestParameter_Parse(t *testing.T) {
	tests := []struct {
		name     string
		attr     string
		number   int
		expected Parameter
	}{
		{
			name:   "simple target and key",
			attr:   "class:title",
			number: 1,
			expected: Parameter{
				Raw:        "class:title",
				Target:     "class",
				Keys:       []string{"title"},
				Conditions: nil,
				Number:     1,
			},
		},
		{
			name:   "target with multiple keys",
			attr:   "data-content:title,author,date",
			number: 2,
			expected: Parameter{
				Raw:        "data-content:title,author,date",
				Target:     "data-content",
				Keys:       []string{"title", "author", "date"},
				Conditions: nil,
				Number:     2,
			},
		},
		{
			name:   "target, key, and conditions",
			attr:   "class:status==published,draft",
			number: 3,
			expected: Parameter{
				Raw:        "class:status==published,draft",
				Target:     "class",
				Keys:       []string{"status"},
				Conditions: []string{"published", "draft"},
				Number:     3,
			},
		},
		{
			name:   "no target (should get auto-generated)",
			attr:   ":title",
			number: 4,
			expected: Parameter{
				Raw:        ":title",
				Target:     "data-attr-4",
				Keys:       []string{"title"},
				Conditions: nil,
				Number:     4,
			},
		},
		{
			name:   "complex attribute with multiple keys and conditions",
			attr:   "data-visibility:status,published==true,yes,1",
			number: 5,
			expected: Parameter{
				Raw:        "data-visibility:status,published==true,yes,1",
				Target:     "data-visibility",
				Keys:       []string{"status", "published"},
				Conditions: []string{"true", "yes", "1"},
				Number:     5,
			},
		},
		{
			name:   "only target without colon",
			attr:   "title",
			number: 6,
			expected: Parameter{
				Raw:        "title",
				Target:     "data-attr-6", // This gets auto-generated because there's no colon
				Keys:       []string{"title"},
				Conditions: nil,
				Number:     6,
			},
		},
		{
			name:   "empty conditions",
			attr:   "class:status==",
			number: 7,
			expected: Parameter{
				Raw:        "class:status==",
				Target:     "class",
				Keys:       []string{"status"},
				Conditions: []string{""},
				Number:     7,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &Parameter{Number: tt.number}
			param.Parse(tt.attr)

			if param.Raw != tt.expected.Raw {
				t.Errorf("Parse() Raw = %v, want %v", param.Raw, tt.expected.Raw)
			}
			if param.Target != tt.expected.Target {
				t.Errorf("Parse() Target = %v, want %v", param.Target, tt.expected.Target)
			}
			if !reflect.DeepEqual(param.Keys, tt.expected.Keys) {
				t.Errorf("Parse() Keys = %v, want %v", param.Keys, tt.expected.Keys)
			}
			if !slicesEqual(param.Conditions, tt.expected.Conditions) {
				t.Errorf("Parse() Conditions = %v, want %v", param.Conditions, tt.expected.Conditions)
			}
			if param.Number != tt.expected.Number {
				t.Errorf("Parse() Number = %v, want %v", param.Number, tt.expected.Number)
			}
		})
	}
}

// TestParameter_Resolve tests parameter resolution with TemplateData
func TestParameter_Resolve(t *testing.T) {
	data := TemplateData{
		"title":  "Test Article",
		"author": "John Doe",
		"status": "published",
		"tags":   []string{"go", "testing"},
		"meta": TemplateData{
			"category": "tech",
			"priority": 1,
		},
	}

	tests := []struct {
		name     string
		keys     []string
		expected []any
	}{
		{
			name:     "single existing key",
			keys:     []string{"title"},
			expected: []any{"Test Article"},
		},
		{
			name:     "multiple existing keys",
			keys:     []string{"title", "author"},
			expected: []any{"Test Article", "John Doe"},
		},
		{
			name:     "mix of existing and non-existing keys",
			keys:     []string{"title", "nonexistent", "status"},
			expected: []any{"Test Article", "published"},
		},
		{
			name:     "all non-existing keys",
			keys:     []string{"nonexistent1", "nonexistent2"},
			expected: nil, // Values will be nil when no keys resolve
		},
		{
			name:     "nested key",
			keys:     []string{"meta.category"},
			expected: []any{"tech"},
		},
		{
			name:     "empty keys",
			keys:     []string{},
			expected: nil, // Values will be nil when no keys are provided
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &Parameter{
				Keys: tt.keys,
			}
			param.Resolve(&data)

			if !reflect.DeepEqual(param.Values, tt.expected) {
				t.Errorf("Resolve() Values = %v, want %v", param.Values, tt.expected)
			}
		})
	}
}

// TestParameter_Value tests getting the first non-nil value
func TestParameter_Value(t *testing.T) {
	tests := []struct {
		name     string
		values   []any
		expected any
	}{
		{
			name:     "first value non-nil",
			values:   []any{"first", "second", "third"},
			expected: "first",
		},
		{
			name:     "first value nil, second non-nil",
			values:   []any{nil, "second", "third"},
			expected: "second",
		},
		{
			name:     "all values nil",
			values:   []any{nil, nil, nil},
			expected: nil,
		},
		{
			name:     "empty values",
			values:   []any{},
			expected: nil,
		},
		{
			name:     "single non-nil value",
			values:   []any{"only"},
			expected: "only",
		},
		{
			name:     "single nil value",
			values:   []any{nil},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &Parameter{
				Values: tt.values,
			}
			got := param.Value()
			if got != tt.expected {
				t.Errorf("Value() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestParameter_ValueSet tests getting all values as a set
func TestParameter_ValueSet(t *testing.T) {
	tests := []struct {
		name     string
		values   []any
		expected []any
	}{
		{
			name:     "string value",
			values:   []any{"test"},
			expected: []any{"test"},
		},
		{
			name:     "slice value",
			values:   []any{[]string{"a", "b", "c"}},
			expected: []any{"a", "b", "c"},
		},
		{
			name:     "array value",
			values:   []any{[3]int{1, 2, 3}},
			expected: []any{1, 2, 3},
		},
		{
			name:     "no value",
			values:   []any{},
			expected: []any{},
		},
		{
			name:     "nil value",
			values:   []any{nil},
			expected: []any{},
		},
		{
			name:     "multiple values (should use first)",
			values:   []any{"first", "second"},
			expected: []any{"first"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &Parameter{
				Values: tt.values,
			}
			got := param.ValueSet()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ValueSet() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestParameter_Evaluate tests parameter condition evaluation
func TestParameter_Evaluate(t *testing.T) {
	tests := []struct {
		name       string
		values     []any
		conditions []string
		expected   bool
	}{
		{
			name:       "no values set",
			values:     []any{},
			conditions: []string{"test"},
			expected:   false,
		},
		{
			name:       "value set, no conditions",
			values:     []any{"test"},
			conditions: []string{},
			expected:   true,
		},
		{
			name:       "value matches condition (string)",
			values:     []any{"published"},
			conditions: []string{"published", "draft"},
			expected:   true,
		},
		{
			name:       "value doesn't match any condition",
			values:     []any{"archived"},
			conditions: []string{"published", "draft"},
			expected:   false,
		},
		{
			name:       "case insensitive matching",
			values:     []any{"Published"},
			conditions: []string{"published"},
			expected:   true,
		},
		{
			name:       "numeric value matching",
			values:     []any{123},
			conditions: []string{"123", "456"},
			expected:   true,
		},
		{
			name:       "boolean value matching",
			values:     []any{true},
			conditions: []string{"true"},
			expected:   true,
		},
		{
			name:       "array value with one matching element",
			values:     []any{[]string{"tag1", "published", "tag3"}},
			conditions: []string{"published"},
			expected:   true,
		},
		{
			name:       "array value with no matching elements",
			values:     []any{[]string{"tag1", "tag2", "tag3"}},
			conditions: []string{"published"},
			expected:   false,
		},
		{
			name:       "multiple values, first one matches",
			values:     []any{"published", "draft"},
			conditions: []string{"published"},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &Parameter{
				Values:     tt.values,
				Conditions: tt.conditions,
			}
			got := param.Evaluate()
			if got != tt.expected {
				t.Errorf("Evaluate() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestParameter_Integration tests the full parameter lifecycle
func TestParameter_Integration(t *testing.T) {
	data := TemplateData{
		"title":  "My Blog Post",
		"status": "published",
		"tags":   []string{"go", "web", "api"},
		"meta": TemplateData{
			"featured": true,
		},
	}

	tests := []struct {
		name           string
		attr           string
		number         int
		expectedTarget string
		expectedEval   bool
	}{
		{
			name:           "simple class assignment",
			attr:           "class:title",
			number:         1,
			expectedTarget: "class",
			expectedEval:   true,
		},
		{
			name:           "conditional visibility based on status",
			attr:           "style:status==published",
			number:         2,
			expectedTarget: "style",
			expectedEval:   true,
		},
		{
			name:           "conditional visibility with non-matching condition",
			attr:           "style:status==draft",
			number:         3,
			expectedTarget: "style",
			expectedEval:   false,
		},
		{
			name:           "tag-based conditional",
			attr:           "data-category:tags==web,mobile",
			number:         4,
			expectedTarget: "data-category",
			expectedEval:   true,
		},
		{
			name:           "nested property conditional",
			attr:           "data-featured:meta.featured==true",
			number:         5,
			expectedTarget: "data-featured",
			expectedEval:   true,
		},
		{
			name:           "multiple keys with OR logic",
			attr:           "class:title,status==My Blog Post,published",
			number:         6,
			expectedTarget: "class",
			expectedEval:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &Parameter{Number: tt.number}

			// Parse the attribute
			param.Parse(tt.attr)

			// Resolve values from data
			param.Resolve(&data)

			// Check target assignment
			if param.Target != tt.expectedTarget {
				t.Errorf("Integration test Target = %v, want %v", param.Target, tt.expectedTarget)
			}

			// Evaluate conditions
			result := param.Evaluate()
			if result != tt.expectedEval {
				t.Errorf("Integration test Evaluate() = %v, want %v", result, tt.expectedEval)
			}
		})
	}
}

// Benchmark tests for performance-critical parameter operations
func BenchmarkParameter_Parse(b *testing.B) {
	attr := "data-visibility:status,category,tags==published,tech,go,web"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		param := &Parameter{Number: i}
		param.Parse(attr)
	}
}

func BenchmarkParameter_Resolve(b *testing.B) {
	data := TemplateData{
		"title":    "Test Article",
		"status":   "published",
		"category": "tech",
		"tags":     []string{"go", "web", "api"},
	}
	param := &Parameter{
		Keys: []string{"title", "status", "category", "tags"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		param.Resolve(&data)
	}
}

func BenchmarkParameter_Evaluate(b *testing.B) {
	param := &Parameter{
		Values:     []any{[]string{"go", "web", "published", "api"}},
		Conditions: []string{"published", "tech"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		param.Evaluate()
	}
}
