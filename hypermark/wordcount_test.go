package hypermark

import (
	"bytes"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	meta "github.com/yuin/goldmark-meta"
)

func TestWordCounter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "simple paragraph",
			input: "Hello world this is a test.",
			want:  6,
		},
		{
			name:  "multiple paragraphs",
			input: "First paragraph here.\n\nSecond paragraph here.",
			want:  6,
		},
		{
			name:  "empty document",
			input: "",
			want:  0,
		},
		{
			name:  "heading and paragraph",
			input: "# My Title\n\nThis is the body text.",
			want:  7,
		},
		{
			name:  "inline formatting bold",
			input: "This is **bold** text.",
			want:  4,
		},
		{
			name:  "inline formatting italic",
			input: "This is _italic_ text.",
			want:  4,
		},
		{
			name:  "link text",
			input: "Visit [my site](https://example.com) today.",
			want:  4,
		},
		{
			name:  "unordered list",
			input: "* one\n* two\n* three",
			want:  3,
		},
		{
			name:  "ordered list",
			input: "1. first\n2. second\n3. third",
			want:  3,
		},
		{
			name:  "code span",
			input: "Use the `fmt.Println` function.",
			want:  4,
		},
		{
			name:  "fenced code block",
			input: "Some text.\n\n```go\nfmt.Println(\"hello\")\n```\n\nMore text.",
			want:  5,
		},
		{
			name:  "blockquote",
			input: "> This is a quote.\n\nAnd this is not.",
			want:  8,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wc := &WordCounter{}
			md := goldmark.New(goldmark.WithExtensions(wc))
			var buf bytes.Buffer
			ctx := parser.NewContext()
			err := md.Convert([]byte(tc.input), &buf, parser.WithContext(ctx))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if wc.Count != tc.want {
				t.Errorf("WordCounter.Count = %d, want %d", wc.Count, tc.want)
			}
		})
	}
}

func TestWordCounterExcludesFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "frontmatter excluded",
			input: "---\ntitle: My Post\ntags:\n  - one\n  - two\n---\n\nThis is the body.",
			want:  4,
		},
		{
			name:  "only frontmatter",
			input: "---\ntitle: Hello World\n---\n",
			want:  0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wc := &WordCounter{}
			md := goldmark.New(goldmark.WithExtensions(meta.Meta, wc))
			var buf bytes.Buffer
			ctx := parser.NewContext()
			err := md.Convert([]byte(tc.input), &buf, parser.WithContext(ctx))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if wc.Count != tc.want {
				t.Errorf("WordCounter.Count = %d, want %d", wc.Count, tc.want)
			}
		})
	}
}

func TestWordCounterContextKey(t *testing.T) {
	ctx := parser.NewContext()
	v := ctx.Get(WordCounterContext)
	if v != nil {
		t.Errorf("WordCounterContext on fresh context = %v, want nil", v)
	}
}

func TestReadingTime(t *testing.T) {
	tests := []struct {
		name  string
		words int
		want  int
	}{
		{name: "zero words", words: 0, want: 0},
		{name: "negative words", words: -1, want: 0},
		{name: "one word", words: 1, want: 1},
		{name: "199 words", words: 199, want: 1},
		{name: "200 words", words: 200, want: 1},
		{name: "201 words", words: 201, want: 2},
		{name: "400 words", words: 400, want: 2},
		{name: "1000 words", words: 1000, want: 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ReadingTime(tc.words)
			if got != tc.want {
				t.Errorf("ReadingTime(%d) = %d, want %d", tc.words, got, tc.want)
			}
		})
	}
}
