package hypertemplates

import (
	"testing"

	"golang.org/x/net/html"
)

// Helper function to create test HTML nodes
func createTestElement(tag string, attrs map[string]string) *html.Node {
	element := &html.Node{
		Type: html.ElementNode,
		Data: tag,
		Attr: []html.Attribute{},
	}
	for key, value := range attrs {
		element.Attr = append(element.Attr, html.Attribute{Key: key, Val: value})
	}
	return element
}

func createTestTextNode(text string) *html.Node {
	return &html.Node{
		Type: html.TextNode,
		Data: text,
	}
}

// TestTemplate_hasAttribute tests the hasAttribute helper function
func TestTemplate_hasAttribute(t *testing.T) {
	tests := []struct {
		name     string
		element  *html.Node
		keys     []string
		expected bool
	}{
		{
			name:     "element has attribute",
			element:  createTestElement("div", map[string]string{"class": "test", "id": "main"}),
			keys:     []string{"class"},
			expected: true,
		},
		{
			name:     "element has one of multiple keys",
			element:  createTestElement("div", map[string]string{"class": "test"}),
			keys:     []string{"id", "class", "style"},
			expected: true,
		},
		{
			name:     "element doesn't have attribute",
			element:  createTestElement("div", map[string]string{"class": "test"}),
			keys:     []string{"id"},
			expected: false,
		},
		{
			name:     "element has no attributes",
			element:  createTestElement("div", map[string]string{}),
			keys:     []string{"class"},
			expected: false,
		},
		{
			name:     "text node (should return false)",
			element:  createTestTextNode("text"),
			keys:     []string{"class"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAttribute(tt.element, tt.keys...)
			if got != tt.expected {
				t.Errorf("hasAttribute() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTemplate_getAttribute tests the getAttribute helper function
func TestTemplate_getAttribute(t *testing.T) {
	tests := []struct {
		name     string
		element  *html.Node
		keys     []string
		expected string
	}{
		{
			name:     "get existing attribute",
			element:  createTestElement("div", map[string]string{"class": "test-class", "id": "main"}),
			keys:     []string{"class"},
			expected: "test-class",
		},
		{
			name:     "get first matching attribute from multiple keys",
			element:  createTestElement("div", map[string]string{"class": "test-class", "id": "main"}),
			keys:     []string{"nonexistent", "class", "id"},
			expected: "test-class",
		},
		{
			name:     "attribute doesn't exist",
			element:  createTestElement("div", map[string]string{"class": "test"}),
			keys:     []string{"id"},
			expected: "",
		},
		{
			name:     "empty attributes",
			element:  createTestElement("div", map[string]string{}),
			keys:     []string{"class"},
			expected: "",
		},
		{
			name:     "text node (should return empty)",
			element:  createTestTextNode("text"),
			keys:     []string{"class"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getAttribute(tt.element, tt.keys...)
			if got != tt.expected {
				t.Errorf("getAttribute() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTemplate_setAttribute tests the setAttribute helper function
func TestTemplate_setAttribute(t *testing.T) {
	tests := []struct {
		name          string
		element       *html.Node
		key           string
		value         string
		expectedAttrs map[string]string
	}{
		{
			name:          "set new attribute",
			element:       createTestElement("div", map[string]string{}),
			key:           "class",
			value:         "test-class",
			expectedAttrs: map[string]string{"class": "test-class"},
		},
		{
			name:          "update existing attribute",
			element:       createTestElement("div", map[string]string{"class": "old-class"}),
			key:           "class",
			value:         "new-class",
			expectedAttrs: map[string]string{"class": "new-class"},
		},
		{
			name:          "add to existing attributes",
			element:       createTestElement("div", map[string]string{"id": "main"}),
			key:           "class",
			value:         "test-class",
			expectedAttrs: map[string]string{"id": "main", "class": "test-class"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip text nodes as setAttribute should guard against them
			if tt.element.Type == html.TextNode {
				return
			}

			setAttribute(tt.element, tt.key, tt.value)

			// Verify all expected attributes are present
			for expectedKey, expectedValue := range tt.expectedAttrs {
				found := false
				for _, attr := range tt.element.Attr {
					if attr.Key == expectedKey {
						if attr.Val != expectedValue {
							t.Errorf("setAttribute() attribute %s = %v, want %v", expectedKey, attr.Val, expectedValue)
						}
						found = true
						break
					}
				}
				if !found {
					t.Errorf("setAttribute() missing expected attribute %s", expectedKey)
				}
			}
		})
	}
}

// TestTemplate_setAttribute_TextNode tests that setAttribute handles text nodes correctly
func TestTemplate_setAttribute_TextNode(t *testing.T) {
	element := createTestTextNode("text")
	originalAttr := element.Attr

	setAttribute(element, "class", "test")

	// Should not modify text nodes
	if len(element.Attr) != len(originalAttr) {
		t.Error("setAttribute() should not modify text nodes")
	}
}

// TestTemplate_innerText tests the innerText helper function
func TestTemplate_innerText(t *testing.T) {
	// Create a more complex HTML structure for testing
	parent := createTestElement("div", map[string]string{})
	child1 := createTestTextNode("Hello ")
	child2 := createTestElement("span", map[string]string{})
	child2.AppendChild(createTestTextNode("world"))
	child3 := createTestTextNode("!")

	parent.AppendChild(child1)
	parent.AppendChild(child2)
	parent.AppendChild(child3)

	tests := []struct {
		name     string
		element  *html.Node
		unescape bool
		expected string
	}{
		{
			name:     "text node returns its data",
			element:  createTestTextNode("simple text"),
			unescape: false,
			expected: "simple text",
		},
		{
			name:     "element with text children",
			element:  parent,
			unescape: false,
			expected: "Hello <span>world</span>!", // innerText returns HTML, not plain text
		},
		{
			name:     "unescape HTML entities",
			element:  createTestTextNode("&lt;test&gt;"),
			unescape: true,
			expected: "<test>",
		},
		{
			name:     "no unescape",
			element:  createTestTextNode("&lt;test&gt;"),
			unescape: false,
			expected: "&lt;test&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := &Template{}
			got := template.innerText(tt.element, tt.unescape)
			if got != tt.expected {
				t.Errorf("innerText() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestTemplate_cloneElement tests the cloneElement helper function
func TestTemplate_cloneElement(t *testing.T) {
	// Create test element with attributes and children
	original := createTestElement("div", map[string]string{"class": "test", "id": "main"})
	child1 := createTestTextNode("Hello")
	child2 := createTestElement("span", map[string]string{"style": "color: red"})
	child2.AppendChild(createTestTextNode("world"))

	original.AppendChild(child1)
	original.AppendChild(child2)

	clone := cloneElement(original)

	// Test basic properties
	if clone.Type != original.Type {
		t.Errorf("cloneElement() Type = %v, want %v", clone.Type, original.Type)
	}
	if clone.Data != original.Data {
		t.Errorf("cloneElement() Data = %v, want %v", clone.Data, original.Data)
	}

	// Test attributes
	if len(clone.Attr) != len(original.Attr) {
		t.Errorf("cloneElement() Attr length = %v, want %v", len(clone.Attr), len(original.Attr))
	}
	for i, attr := range original.Attr {
		if clone.Attr[i].Key != attr.Key || clone.Attr[i].Val != attr.Val {
			t.Errorf("cloneElement() Attr[%d] = %v, want %v", i, clone.Attr[i], attr)
		}
	}

	// Test that it's a different object (not same memory reference)
	if clone == original {
		t.Error("cloneElement() should return a different object")
	}

	// Test children are cloned recursively
	originalChildren := childNodes(original)
	clonedChildren := childNodes(clone)

	if len(clonedChildren) != len(originalChildren) {
		t.Errorf("cloneElement() children length = %v, want %v", len(clonedChildren), len(originalChildren))
	}

	// Verify children are different objects but have same content
	for i, originalChild := range originalChildren {
		clonedChild := clonedChildren[i]
		if clonedChild == originalChild {
			t.Errorf("cloneElement() child %d should be different object", i)
		}
		if clonedChild.Type != originalChild.Type || clonedChild.Data != originalChild.Data {
			t.Errorf("cloneElement() child %d content differs", i)
		}
	}
}

// TestTemplate_childNodes tests the childNodes helper function
func TestTemplate_childNodes(t *testing.T) {
	tests := []struct {
		name         string
		setupElement func() *html.Node
		expectedLen  int
	}{
		{
			name: "element with no children",
			setupElement: func() *html.Node {
				return createTestElement("div", map[string]string{})
			},
			expectedLen: 0,
		},
		{
			name: "element with text child",
			setupElement: func() *html.Node {
				parent := createTestElement("div", map[string]string{})
				parent.AppendChild(createTestTextNode("text"))
				return parent
			},
			expectedLen: 1,
		},
		{
			name: "element with multiple children",
			setupElement: func() *html.Node {
				parent := createTestElement("div", map[string]string{})
				parent.AppendChild(createTestTextNode("text1"))
				parent.AppendChild(createTestElement("span", map[string]string{}))
				parent.AppendChild(createTestTextNode("text2"))
				return parent
			},
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			element := tt.setupElement()
			children := childNodes(element)

			if len(children) != tt.expectedLen {
				t.Errorf("childNodes() length = %v, want %v", len(children), tt.expectedLen)
			}

			// Verify that returned children are actually children of the element
			for i, child := range children {
				if child.Parent != element {
					t.Errorf("childNodes()[%d] parent is not the element", i)
				}
			}
		})
	}
}

// TestTemplate_IsAbsoluteHref tests the URL validation method
func TestTemplate_IsAbsoluteHref(t *testing.T) {
	template := &Template{}

	tests := []struct {
		name     string
		href     string
		expected bool
	}{
		{
			name:     "http URL",
			href:     "http://example.com/path",
			expected: true,
		},
		{
			name:     "https URL",
			href:     "https://example.com/path",
			expected: true,
		},
		{
			name:     "protocol relative URL",
			href:     "//example.com/path",
			expected: true,
		},
		{
			name:     "absolute path",
			href:     "/path/to/resource",
			expected: true,
		},
		{
			name:     "relative path",
			href:     "path/to/resource",
			expected: false,
		},
		{
			name:     "relative path with dot",
			href:     "./path/to/resource",
			expected: false,
		},
		{
			name:     "relative path with double dot",
			href:     "../path/to/resource",
			expected: false,
		},
		{
			name:     "empty string",
			href:     "",
			expected: false,
		},
		{
			name:     "fragment only",
			href:     "#section",
			expected: false,
		},
		{
			name:     "query only",
			href:     "?param=value",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := template.IsAbsoluteHref(tt.href)
			if got != tt.expected {
				t.Errorf("IsAbsoluteHref(%v) = %v, want %v", tt.href, got, tt.expected)
			}
		})
	}
}

// TestTemplate_ParseParameters tests the parameter parsing functionality
func TestTemplate_ParseParameters(t *testing.T) {
	template := &Template{}
	data := TemplateData{
		"title":  "Test",
		"status": "published",
		"tags":   []string{"go", "test"},
	}

	tests := []struct {
		name        string
		attr        string
		expectedLen int
	}{
		{
			name:        "empty attribute string",
			attr:        "",
			expectedLen: 0,
		},
		{
			name:        "single parameter",
			attr:        "class:title",
			expectedLen: 1,
		},
		{
			name:        "multiple parameters",
			attr:        "class:title; style:status==published; data-tags:tags",
			expectedLen: 3,
		},
		{
			name:        "parameters with whitespace",
			attr:        " class:title ; style:status==published ",
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := template.ParseParameters(tt.attr, &data)
			if len(params) != tt.expectedLen {
				t.Errorf("ParseParameters() length = %v, want %v", len(params), tt.expectedLen)
			}

			// Verify each parameter was parsed and resolved
			for i, param := range params {
				if param == nil {
					t.Errorf("ParseParameters()[%d] is nil", i)
				}
				if len(param.Raw) == 0 {
					t.Errorf("ParseParameters()[%d] was not parsed", i)
				}
			}
		})
	}
}

// TestTemplate_ParseParameter tests single parameter parsing
func TestTemplate_ParseParameter(t *testing.T) {
	template := &Template{}
	data := TemplateData{
		"title":  "Test Article",
		"status": "published",
	}

	tests := []struct {
		name           string
		attr           string
		expectedTarget string
		expectedEval   bool
	}{
		{
			name:           "simple parameter",
			attr:           "class:title",
			expectedTarget: "class",
			expectedEval:   true,
		},
		{
			name:           "conditional parameter that matches",
			attr:           "style:status==published",
			expectedTarget: "style",
			expectedEval:   true,
		},
		{
			name:           "conditional parameter that doesn't match",
			attr:           "style:status==draft",
			expectedTarget: "style",
			expectedEval:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := template.ParseParameter(tt.attr, &data)

			if param.Target != tt.expectedTarget {
				t.Errorf("ParseParameter() Target = %v, want %v", param.Target, tt.expectedTarget)
			}

			if param.Evaluate() != tt.expectedEval {
				t.Errorf("ParseParameter() Evaluate() = %v, want %v", param.Evaluate(), tt.expectedEval)
			}
		})
	}
}

// TestTemplate_AttributeData tests converting element attributes to TemplateData
func TestTemplate_AttributeData(t *testing.T) {
	template := &Template{}

	tests := []struct {
		name     string
		element  *html.Node
		expected TemplateData
	}{
		{
			name:     "element with attributes",
			element:  createTestElement("div", map[string]string{"class": "test", "id": "main", "data-value": "123"}),
			expected: TemplateData{"class": "test", "id": "main", "data-value": "123"},
		},
		{
			name:     "element with no attributes",
			element:  createTestElement("div", map[string]string{}),
			expected: TemplateData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := template.AttributeData(tt.element)

			// Check that all expected keys are present with correct values
			for expectedKey, expectedValue := range tt.expected {
				if gotValue, exists := got[expectedKey]; !exists {
					t.Errorf("AttributeData() missing key %v", expectedKey)
				} else if gotValue != expectedValue {
					t.Errorf("AttributeData()[%v] = %v, want %v", expectedKey, gotValue, expectedValue)
				}
			}

			// Check that no extra keys are present
			for gotKey := range got {
				if _, exists := tt.expected[gotKey]; !exists {
					t.Errorf("AttributeData() unexpected key %v", gotKey)
				}
			}
		})
	}
}

// Benchmark tests for performance-critical template operations
func BenchmarkTemplate_getAttribute(b *testing.B) {
	element := createTestElement("div", map[string]string{
		"class":      "test-class",
		"id":         "main",
		"style":      "color: red",
		"data-value": "123",
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getAttribute(element, "class", "id", "nonexistent")
	}
}

func BenchmarkTemplate_setAttribute(b *testing.B) {
	element := createTestElement("div", map[string]string{"class": "old"})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		setAttribute(element, "class", "new-class")
	}
}

func BenchmarkTemplate_cloneElement(b *testing.B) {
	// Create a reasonably complex element for benchmarking
	parent := createTestElement("div", map[string]string{"class": "test", "id": "main"})
	for i := 0; i < 5; i++ {
		child := createTestElement("span", map[string]string{"data-index": string(rune('0' + i))})
		child.AppendChild(createTestTextNode("text"))
		parent.AppendChild(child)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloneElement(parent)
	}
}
