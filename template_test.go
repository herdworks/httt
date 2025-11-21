package hypertemplates

import (
	"bytes"
	"errors"
	"net/url"
	"os"
	"testing"

	"golang.org/x/net/html"
)

// Template.ResolveIncludes
// Ensure that ht-include elements are properly resolved.
func TestResolveIncludes(t *testing.T) {
	tests := []struct {
		name   string
		layout string
		theme  string
		err    error
	}{
		{
			name:   "resolve includes",
			layout: "testdata/ht-include/layouts/ok.html",
			theme:  "testdata/ht-include/theme.json",
			err:    nil,
		},
		{
			name:   "broken include",
			layout: "testdata/ht-include/layouts/broken.html",
			theme:  "testdata/ht-include/theme.json",
			err:    ErrIncludeFragment,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var layoutBytes []byte
			var layoutHTML *html.Node
			var theme *HyperTheme = &HyperTheme{URI: &url.URL{Path: tc.theme}}
			var err error
			if layoutBytes, err = os.ReadFile(tc.layout); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("unexpected error reading layout %q bytes:\n\twanted: %v\n\t   got: %v", tc.layout, tc.err, err)
			}
			if layoutHTML, err = html.Parse(bytes.NewReader(layoutBytes)); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("failed parsing layout %q html: %v", tc.layout, err)
			}
			template := Template{Layout: layoutHTML, Theme: theme}
			if template.Layout == nil {
				t.Fatalf("template.Layout is nil")
			}
			if err = template.ResolveIncludes(); tc.err != nil && errors.Is(err, tc.err) {
				return // expected error
			} else if err != nil {
				t.Fatalf("error resolving includes: %v", err)
			}
			if tc.err != nil {
				t.Fatalf("expected error %q but got none", tc.err)
			}
			// t.Logf("\n%s\n", template.HTML(template.Layout))
		})
	}
}

// TestAttributeForwarding
// Ensure that attributes are forwarded from ht-include placeholder elements to an included fragment.
func TestAttributeForwarding(t *testing.T) {
	tests := []struct {
		name   string
		layout string
		theme  string
	}{
		{
			name:   "forward attributes",
			theme:  "testdata/ht-include/theme.json",
			layout: "testdata/ht-include/layouts/ok.html",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var theme *HyperTheme = &HyperTheme{URI: &url.URL{Path: tc.theme}}
			var layoutBytes []byte
			var layoutHTML *html.Node
			var err error
			if layoutBytes, err = os.ReadFile(tc.layout); err != nil {
				t.Fatalf("error reading layout %q bytes: %v", tc.layout, err)
			}
			if layoutHTML, err = html.Parse(bytes.NewReader(layoutBytes)); err != nil {
				t.Fatalf("error parsing layout %q html: %v", tc.layout, err)
			}
			template := Template{Layout: layoutHTML, Theme: theme}
			if template.Layout == nil {
				t.Fatalf("template.Layout is nil")
			}
			if err = template.ResolveIncludes(); err != nil {
				t.Fatalf("error resolving includes: %v", err)
			}
			if matches := QueryAll(template.Layout, "header#hero"); len(matches) != 1 {
				t.Fatalf("expected 1 match for header#hero, got %d", len(matches))
			}
			if matches := QueryAll(template.Layout, "footer#forwarded"); len(matches) != 1 {
				t.Fatalf("expected 1 match for footer#forwarded, got %d", len(matches))
			}
		})
	}
}

// func TestHTMLAttributeParser(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		fragment []byte
// 	}{
// 		{
// 			name:     "duplicate attributes with the same key",
// 			fragment: []byte("<div ht-if='a' ht-if='b'>Test</div>"),
// 		},
// 	}
// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			var fragments []*html.Node
// 			var err error
// 			var parent *html.Node = &html.Node{Type: html.ElementNode, Data: "test-element"}
// 			if fragments, err = html.ParseFragment(bytes.NewReader(tc.fragment), parent); err != nil {
// 				t.Errorf("ParseFragment() error = %v", err)
// 			}
// 			for _, fragment := range fragments {
// 				for i, attr := range fragment.Attr {
// 					t.Logf("[%d] key: %s, value: %s", i, attr.Key, attr.Val)
// 				}
// 			}
// 		})
// 	}
// }
