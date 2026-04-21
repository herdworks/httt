package hypermark

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
)

func TestHasHTBlockAttr(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "standalone bool attr", in: `link-attachment ht-block`, want: true},
		{name: "attr with value", in: `link-attachment ht-block="true" href="/x"`, want: true},
		{name: "attr among many", in: `link-attachment href="/x" ht-block class="foo"`, want: true},
		{name: "attr at end before slash", in: `link-attachment ht-block/`, want: true},
		{name: "attr newline-separated", in: "link-attachment\n  ht-block\n  href=\"/x\"", want: true},
		{name: "attr tab-separated", in: "link-attachment\tht-block\thref=\"/x\"", want: true},

		{name: "no ht-block at all", in: `link-attachment href="/x"`, want: false},
		{name: "inside double-quoted value", in: `link-attachment data-note="see ht-block docs"`, want: false},
		{name: "inside single-quoted value", in: `link-attachment data-note='see ht-block docs'`, want: false},
		{name: "prefix of longer attr name", in: `link-attachment ht-block-more`, want: false},
		{name: "only in tag name", in: `ht-block`, want: false},
		{name: "adjacent without whitespace", in: `link-attachmentht-block`, want: false},
		{name: "inside value with equals", in: `link-attachment data-x="ht-block=1"`, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasHTBlockAttr([]byte(tc.in))
			if got != tc.want {
				t.Errorf("hasHTBlockAttr(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestPeekOpeningTag(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		want     string
		wantOK   bool
		maxLines int
	}{
		{
			name:     "single line simple",
			src:      `<my-elem attr="x">body`,
			want:     `my-elem attr="x"`,
			wantOK:   true,
			maxLines: 32,
		},
		{
			name:     "multi-line attrs",
			src:      "<my-elem\n  ht-block\n  href=\"/x\"\n>body",
			want:     "my-elem\n  ht-block\n  href=\"/x\"\n",
			wantOK:   true,
			maxLines: 32,
		},
		{
			name:     "gt inside double-quoted value",
			src:      `<my-elem title="a > b">body`,
			want:     `my-elem title="a > b"`,
			wantOK:   true,
			maxLines: 32,
		},
		{
			name:     "gt inside single-quoted value",
			src:      `<my-elem title='a > b'>body`,
			want:     `my-elem title='a > b'`,
			wantOK:   true,
			maxLines: 32,
		},
		{
			name:     "unterminated bails",
			src:      `<my-elem attr="x"`,
			wantOK:   false,
			maxLines: 32,
		},
		{
			name:     "exceeds maxLines",
			src:      "<my-elem\n\n\n\n>",
			wantOK:   false,
			maxLines: 2,
		},
		{
			name:     "stray < bails",
			src:      `<my-elem <nested>`,
			wantOK:   false,
			maxLines: 32,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := peekOpeningTag([]byte(tc.src), 0, tc.maxLines)
			if ok != tc.wantOK {
				t.Fatalf("peekOpeningTag ok = %v, want %v (got=%q)", ok, tc.wantOK, string(got))
			}
			if ok && string(got) != tc.want {
				t.Errorf("peekOpeningTag = %q, want %q", string(got), tc.want)
			}
		})
	}
}

// renderMarkdown runs the ht-block extension (only) over the given input and
// returns the rendered HTML. Uses Unsafe so raw HTML is preserved.
func renderMarkdown(t *testing.T, input string) string {
	t.Helper()
	md := goldmark.New(
		goldmark.WithExtensions(HTBlockHTMLParserExtension()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(input), &buf); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	return buf.String()
}

func TestHTBlockExtension_BlockVsInline(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContains []string
		notContains  []string
	}{
		{
			name: "block single-line open tag",
			input: "Intro paragraph.\n\n" +
				`<link-attachment ht-block href="/x"></link-attachment>` + "\n\n" +
				"Outro paragraph.\n",
			wantContains: []string{
				`<link-attachment ht-block href="/x"></link-attachment>`,
			},
			notContains: []string{
				`<p><link-attachment`,
			},
		},
		{
			name: "block multi-line open tag",
			input: "Intro paragraph.\n\n" +
				"<link-attachment\n" +
				"  ht-block\n" +
				`  href="/x"` + "\n" +
				"></link-attachment>\n\n" +
				"Outro paragraph.\n",
			notContains: []string{
				`<p><link-attachment`,
			},
		},
		{
			name:  "inline usage stays in paragraph",
			input: `See <icon-thing ht-block></icon-thing> here.` + "\n",
			wantContains: []string{
				`<p>See <icon-thing ht-block></icon-thing> here.</p>`,
			},
		},
		{
			name: "interrupts preceding paragraph",
			input: "Prose line with no blank line after.\n" +
				`<link-attachment ht-block href="/x"></link-attachment>` + "\n",
			wantContains: []string{
				`<p>Prose line with no blank line after.</p>`,
				`<link-attachment ht-block href="/x"></link-attachment>`,
			},
			notContains: []string{
				`<p>Prose line with no blank line after.` + "\n" + `<link-attachment`,
			},
		},
		{
			name: "custom element without ht-block is not promoted",
			input: "Intro.\n\n" +
				"<my-widget\n  class=\"foo\"\n></my-widget>\n\n" +
				"Outro.\n",
			wantContains: []string{
				`&lt;my-widget`,
			},
			notContains: []string{
				"<my-widget\n  class=\"foo\"\n></my-widget>",
			},
		},
		{
			name: "ht-block only in attribute value is not promoted",
			input: "Intro.\n\n" +
				`<link-attachment data-note="see ht-block"></link-attachment>` + "\n\n" +
				"Outro.\n",
			wantContains: []string{
				`<p><link-attachment`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := renderMarkdown(t, tc.input)
			for _, want := range tc.wantContains {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q\nOutput:\n%s", want, out)
				}
			}
			for _, bad := range tc.notContains {
				if strings.Contains(out, bad) {
					t.Errorf("output unexpectedly contains %q\nOutput:\n%s", bad, out)
				}
			}
		})
	}
}
