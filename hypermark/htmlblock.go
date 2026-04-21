package hypermark

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// htBlockAttribute is the opt-in attribute name that promotes a custom
// element to a CommonMark type-6 HTML block.
const htBlockAttribute = "ht-block"

// htBlockMaxLookahead bounds how many lines the opening-tag scanner may
// traverse before giving up. Opening tags longer than this fall through to
// goldmark's default behavior.
const htBlockMaxLookahead = 32

// htBlockStartRegexp is a fast prefix reject for the start condition: a
// hyphenated custom-element tag at line start (≤3 space indent). The full
// opening-tag body — including attribute validation — is scanned separately
// via peekOpeningTag and hasHTBlockAttr to correctly handle multi-line
// opening tags and quoted attribute values.
var htBlockStartRegexp = regexp.MustCompile(
	`^[ ]{0,3}<(?:/[ ]*)?[a-zA-Z][a-zA-Z0-9]*-[a-zA-Z0-9\-]*`,
)

// HTBlockHTMLParserExtension returns a goldmark extender that registers a
// block parser in front of goldmark's default HTMLBlockParser. Hyphenated
// custom elements carrying the `ht-block` attribute on their opening tag
// are promoted to CommonMark type-6 HTML blocks so they render as siblings
// rather than being wrapped in a paragraph.
//
// CommonMark type-6 ordinarily requires tag names in a fixed whitelist, and
// type-7 requires the opening tag to fit on one line. Custom elements with
// multi-line attribute formatting satisfy neither, so goldmark falls back
// to inline HTML inside a `<p>`. This extension fills that gap.
func HTBlockHTMLParserExtension() goldmark.Extender { return &htBlockExtender{} }

type htBlockExtender struct{}

func (e *htBlockExtender) Extend(md goldmark.Markdown) {
	md.Parser().AddOptions(parser.WithBlockParsers(
		util.Prioritized(&htBlockHTMLParser{delegate: parser.NewHTMLBlockParser()}, 499),
	))
}

// htBlockHTMLParser implements parser.BlockParser. It reuses goldmark's
// default HTMLBlockParser for Continue and Close — the block state machine
// is unchanged; only the start condition is extended.
type htBlockHTMLParser struct {
	delegate parser.BlockParser
}

func (b *htBlockHTMLParser) Trigger() []byte { return []byte{'<'} }

func (b *htBlockHTMLParser) Open(parent ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) {
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 || pos >= len(line) || line[pos] != '<' {
		return nil, parser.NoChildren
	}
	if !htBlockStartRegexp.Match(line) {
		return nil, parser.NoChildren
	}

	openPos := segment.Start + pos
	openingTag, ok := peekOpeningTag(reader.Source(), openPos, htBlockMaxLookahead)
	if !ok {
		return nil, parser.NoChildren
	}
	if !hasHTBlockAttr(openingTag) {
		return nil, parser.NoChildren
	}

	node := ast.NewHTMLBlock(ast.HTMLBlockType6)
	reader.Advance(segment.Len() - util.TrimRightSpaceLength(line))
	node.Lines().Append(segment)
	return node, parser.NoChildren
}

// Continue delegates to goldmark's default HTMLBlockParser. The node we
// create in Open carries HTMLBlockType6, so the delegate runs the exact
// type-6 logic: consume lines until a blank line, then close.
func (b *htBlockHTMLParser) Continue(node ast.Node, reader text.Reader, pc parser.Context) parser.State {
	return b.delegate.Continue(node, reader, pc)
}

// Close delegates to goldmark's default HTMLBlockParser, which is a no-op
// for type-6 blocks.
func (b *htBlockHTMLParser) Close(node ast.Node, reader text.Reader, pc parser.Context) {
	b.delegate.Close(node, reader, pc)
}

func (b *htBlockHTMLParser) CanInterruptParagraph() bool { return true }
func (b *htBlockHTMLParser) CanAcceptIndentedLine() bool { return false }

// peekOpeningTag scans forward from the `<` at openPos in src looking for
// the matching `>` that closes the opening tag. Quoted attribute values —
// `"..."` and `'...'` — are skipped so a `>` appearing inside a value does
// not terminate the scan. Returns the opening-tag bytes between `<` and
// `>` (exclusive of both) and true on success.
//
// Returns false if more than maxLines newlines are traversed before `>` is
// found, if EOF is reached first, or if a stray `<` is seen outside any
// quoted region (which would indicate malformed markup).
func peekOpeningTag(src []byte, openPos, maxLines int) ([]byte, bool) {
	if openPos >= len(src) || src[openPos] != '<' {
		return nil, false
	}
	lines := 0
	inDoubleQuote := false
	inSingleQuote := false
	for i := openPos + 1; i < len(src); i++ {
		c := src[i]
		if inDoubleQuote {
			if c == '"' {
				inDoubleQuote = false
			}
			continue
		}
		if inSingleQuote {
			if c == '\'' {
				inSingleQuote = false
			}
			continue
		}
		switch c {
		case '"':
			inDoubleQuote = true
		case '\'':
			inSingleQuote = true
		case '>':
			return src[openPos+1 : i], true
		case '<':
			return nil, false
		case '\n':
			lines++
			if lines > maxLines {
				return nil, false
			}
		}
	}
	return nil, false
}

// hasHTBlockAttr reports whether the opening-tag body contains `ht-block`
// as a standalone attribute name. Quoted regions are skipped so that
// `ht-block` appearing as a substring inside an attribute value (e.g.
// `data-note="see ht-block docs"`) does not match.
//
// The attribute name is recognized when preceded by whitespace and followed
// by whitespace, `=`, `/`, `>`, or end of the opening-tag body.
func hasHTBlockAttr(openingTag []byte) bool {
	needle := []byte(htBlockAttribute)
	inDoubleQuote := false
	inSingleQuote := false
	for i := 0; i < len(openingTag); i++ {
		c := openingTag[i]
		if inDoubleQuote {
			if c == '"' {
				inDoubleQuote = false
			}
			continue
		}
		if inSingleQuote {
			if c == '\'' {
				inSingleQuote = false
			}
			continue
		}
		switch c {
		case '"':
			inDoubleQuote = true
		case '\'':
			inSingleQuote = true
		case ' ', '\t', '\n', '\r':
			j := i + 1
			for j < len(openingTag) && isTagWhitespace(openingTag[j]) {
				j++
			}
			if j+len(needle) <= len(openingTag) && bytes.Equal(openingTag[j:j+len(needle)], needle) {
				if j+len(needle) == len(openingTag) {
					return true
				}
				next := openingTag[j+len(needle)]
				if isTagWhitespace(next) || next == '=' || next == '/' || next == '>' {
					return true
				}
			}
			i = j - 1
		}
	}
	return false
}

func isTagWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
