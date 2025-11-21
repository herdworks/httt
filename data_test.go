package hypertemplates

import (
	"encoding/json"
	"errors"
	"os"
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
