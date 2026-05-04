package markdown

import (
	"strings"
	"testing"
)

func TestSummarizeContent(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		summary string
		more    string
	}{
		{"no delimiter", "<p>just a paragraph</p>", "", ""},
		{"canonical", "<p>before</p><!--more--><p>after</p>", "<p>before</p>", "<p>after</p>"},
		{"single-space variant", "<p>before</p><!-- more --><p>after</p>", "<p>before</p>", "<p>after</p>"},
		{"multi-space variant", "<p>before</p><!--   more    --><p>after</p>", "<p>before</p>", "<p>after</p>"},
		{"tab and newline variant", "<p>before</p><!--\n\tmore \t--><p>after</p>", "<p>before</p>", "<p>after</p>"},
		{"near-miss does not split", "<p>before</p><!--morex--><p>after</p>", "", ""},
		{"two delimiters splits on first only", "<p>a</p><!--more--><p>b</p><!-- more --><p>c</p>", "<p>a</p>", "<p>b</p><!-- more --><p>c</p>"},
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

const sampleMarkdown = `---
title: Hello
tags:
  - one
  - two
---

# Heading

This is a paragraph with #hashtag and @mention.

<!-- more -->

After the fold.
`

func TestParse(t *testing.T) {
	md := New(nil)
	if err := md.Parse([]byte(sampleMarkdown)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if md.FrontMatter["title"] != "Hello" {
		t.Errorf("front-matter title = %v, want Hello", md.FrontMatter["title"])
	}
	if !strings.Contains(md.Content, "<h1") {
		t.Errorf("content missing <h1>: %q", md.Content)
	}
	if md.Summary == "" || md.More == "" {
		t.Errorf("summary/more not split by <!-- more --> delimiter")
	}
	if md.WordCount == 0 {
		t.Errorf("expected non-zero word count")
	}
}

func TestTemplateData(t *testing.T) {
	md := New(nil)
	if err := md.Parse([]byte(sampleMarkdown)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	d := md.TemplateData()
	if d.Get("title") != "Hello" {
		t.Errorf("title = %v, want Hello", d.Get("title"))
	}
	if !strings.Contains(d.Get("content").(string), "<h1") {
		t.Errorf("content missing <h1>")
	}
	if d.Get("summary") == nil || d.Get("more") == nil {
		t.Errorf("summary/more not present in TemplateData")
	}
	if d.Get("wordcount") == nil {
		t.Errorf("wordcount not present in TemplateData")
	}
}

func TestUnmarshalTemplateData(t *testing.T) {
	d, err := UnmarshalTemplateData([]byte(sampleMarkdown), nil)
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	if d.Get("title") != "Hello" {
		t.Errorf("title = %v, want Hello", d.Get("title"))
	}
}

func TestFrontMatterStringifiesKeys(t *testing.T) {
	src := `---
nested:
  key: value
---

body
`
	d, err := UnmarshalTemplateData([]byte(src), nil)
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	// Should resolve via Get without issues — i.e., the map[any]any from
	// goldmark-meta must have been normalized.
	if got := d.Get("nested.key"); got != "value" {
		t.Fatalf("nested.key = %v, want value", got)
	}
}
