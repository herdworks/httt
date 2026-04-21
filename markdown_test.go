package hypertemplates

import (
	"testing"
)

// TestHyperMarkSummarizeContent
// Ensure that SummarizeContent splits on <!--more--> and its whitespace variants,
// and leaves content unsplit when the delimiter is absent or malformed.
func TestHyperMarkSummarizeContent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		summary string
		more    string
	}{
		{
			name:    "no delimiter",
			input:   "<p>just a paragraph</p>",
			summary: "",
			more:    "",
		},
		{
			name:    "canonical",
			input:   "<p>before</p><!--more--><p>after</p>",
			summary: "<p>before</p>",
			more:    "<p>after</p>",
		},
		{
			name:    "single-space variant",
			input:   "<p>before</p><!-- more --><p>after</p>",
			summary: "<p>before</p>",
			more:    "<p>after</p>",
		},
		{
			name:    "multi-space variant",
			input:   "<p>before</p><!--   more    --><p>after</p>",
			summary: "<p>before</p>",
			more:    "<p>after</p>",
		},
		{
			name:    "tab and newline variant",
			input:   "<p>before</p><!--\n\tmore \t--><p>after</p>",
			summary: "<p>before</p>",
			more:    "<p>after</p>",
		},
		{
			name:    "near-miss does not split",
			input:   "<p>before</p><!--morex--><p>after</p>",
			summary: "",
			more:    "",
		},
		{
			name:    "two delimiters splits on first only",
			input:   "<p>a</p><!--more--><p>b</p><!-- more --><p>c</p>",
			summary: "<p>a</p>",
			more:    "<p>b</p><!-- more --><p>c</p>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			md := &HyperMark{}
			md.SummarizeContent([]byte(tc.input))
			if md.Content != tc.input {
				t.Fatalf("Content mismatch: want %q, got %q", tc.input, md.Content)
			}
			if md.Summary != tc.summary {
				t.Fatalf("Summary mismatch: want %q, got %q", tc.summary, md.Summary)
			}
			if md.More != tc.more {
				t.Fatalf("More mismatch: want %q, got %q", tc.more, md.More)
			}
		})
	}
}
