package directives

import (
	"bytes"
	"testing"

	"golang.org/x/net/html"
)

func TestTemplateParameterParser(t *testing.T) {
	tests := []struct {
		name      string
		attribute string
		fragment  []byte
		data      TemplateData
		operator  Operator
		keys      []string
		values    []any
	}{
		{
			name:      "ht-include param",
			attribute: "ht-include",
			fragment:  []byte("<header ht-include='partials/header'>Test</header>"),
			keys:      []string{"partials/header"},
		},
		{
			name:      "ht-if param key exists",
			attribute: "ht-if",
			fragment:  []byte("<div ht-if='user,active'>Test</div>"),
			data:      TemplateData{"user": nil, "active": nil},
			keys:      []string{"user", "active"},
		},
		{
			name:      "ht-if param values exist",
			attribute: "ht-if",
			fragment:  []byte("<div ht-if='user,active'>Test</div>"),
			data:      TemplateData{"user": "jane", "active": true},
			keys:      []string{"user", "active"},
			values:    []any{"jane", true},
		},
		{
			name:      "ht-if equal to operator",
			attribute: "ht-if",
			fragment:  []byte("<div ht-if='user,active=='>Test</div>"),
			data:      TemplateData{"user": nil, "active": nil},
			operator:  OperatorEquals,
			keys:      []string{"user", "active"},
		},
		{
			name:      "ht-if not equal to operator",
			attribute: "ht-if",
			fragment:  []byte("<div ht-if='user,active!='>Test</div>"),
			data:      TemplateData{"user": nil, "active": nil},
			operator:  OperatorNotEquals,
			keys:      []string{"user", "active"},
		},
		{
			name:      "ht-if regular expression operator",
			attribute: "ht-if",
			fragment:  []byte("<div ht-if='user,active~='>Test</div>"),
			data:      TemplateData{"user": nil, "active": nil},
			operator:  OperatorRegexMatch,
			keys:      []string{"user", "active"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var fragments []*html.Node
			var attribute html.Attribute
			var parameter Parameter = Parameter{}
			var err error
			var parent *html.Node = &html.Node{Type: html.ElementNode, Data: "test-element"}
			if fragments, err = html.ParseFragment(bytes.NewReader(tc.fragment), parent); err != nil {
				t.Errorf("error parsing html fragment: %v", err)
			}
			for _, fragment := range fragments {
				for _, attr := range fragment.Attr {
					if attr.Key == tc.attribute {
						attribute = attr
						break
					}
				}
			}
			parameter.Parse(attribute.Val)
			parameter.Resolve(&tc.data)
			if tc.operator != parameter.Operator {
				t.Fatalf("expected operator %v, got %v", tc.operator, parameter.Operator)
			}

			if len(tc.keys) != len(parameter.Keys) {
				t.Fatalf("expected %d keys, got %d", len(tc.keys), len(parameter.Keys))
			}
			for i := range tc.keys {
				if tc.keys[i] != parameter.Keys[i] {
					t.Fatalf("expected key %v at index %d, got %v", tc.keys[i], i, parameter.Keys[i])
				}
			}

			if len(tc.values) != len(parameter.Values) {
				t.Fatalf("expected %d values, got %d", len(tc.values), len(parameter.Values))
			}
			for i := range tc.values {
				if tc.values[i] != parameter.Values[i] {
					t.Fatalf("expected value %v at index %d, got %v", tc.values[i], i, parameter.Values[i])
				}
			}
		})
	}
}
