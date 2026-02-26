package hypermark

import (
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

func WordCounterExtension() *WordCounter {
	return &WordCounter{}
}

// WordCounter is a Goldmark extension that counts words in the body of a
// markdown document, excluding YAML frontmatter.
type WordCounter struct {
	Count int
}
type WordCount int

// WordCounterContext is the parser context key for storing the word count.
var WordCounterContext parser.ContextKey = parser.NewContextKey()

// Extend implements goldmark.Extender.
func (wc *WordCounter) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(&wordCounterTransformer{counter: wc}, 999),
		),
	)
}

// wordCounterTransformer walks the AST after parsing and counts words in
// text nodes. It stores the result in the parser context and updates the
// WordCounter.Count field.
type wordCounterTransformer struct {
	counter *WordCounter
}

// Transform implements parser.ASTTransformer.
func (t *wordCounterTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	count := 0

	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Text:
			seg := v.Segment
			if seg.Len() > 0 {
				count += len(strings.Fields(string(seg.Value(source))))
			}
		case *ast.String:
			count += len(strings.Fields(string(v.Value)))
		case *ast.CodeSpan:
			// Count words in child text nodes, then skip children
			// to avoid double-counting.
			for child := v.FirstChild(); child != nil; child = child.NextSibling() {
				if t, ok := child.(*ast.Text); ok {
					seg := t.Segment
					if seg.Len() > 0 {
						count += len(strings.Fields(string(seg.Value(source))))
					}
				}
			}
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			count += countBlockLines(n, source)
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	t.counter.Count = count
	pc.Set(WordCounterContext, count)
}

// countBlockLines counts words across all line segments of a block node.
func countBlockLines(n ast.Node, source []byte) int {
	count := 0
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		count += len(strings.Fields(string(seg.Value(source))))
	}
	return count
}

// ReadingTime estimates the reading time in minutes based on word count,
// using the commonly cited average of 200 words per minute.
func ReadingTime(words int) int {
	if words <= 0 {
		return 0
	}
	// Round up by adding (wpm-1) before dividing.
	wpm := 200
	return (words + wpm - 1) / wpm
}

// RuneCount returns the number of runes (Unicode code points) in a byte slice.
// RuneCount is a convenience wrapper around unicode/utf8.RuneCount.
func RuneCount(b []byte) int {
	return utf8.RuneCount(b)
}
