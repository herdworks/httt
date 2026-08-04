// Package markdown provides the HyperTemplates markdown parser. It wraps
// goldmark with the GFM, footnote, definition-list, emoji, and YAML
// front-matter extensions, plus the project-specific hypermark extensions
// (mentions, hashtags, word counter, ht-block HTML parsing) and Chroma syntax
// highlighting.
//
// The package owns the goldmark dependency tree so that callers who do not
// need markdown parsing can avoid pulling in goldmark, chroma, and the meta
// parser. To produce a TemplateData from markdown bytes (handling the
// goldmark-meta yaml.v2 front-matter quirk), use UnmarshalTemplateData.
package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/herdworks/hypertemplates-go/data"
	"github.com/herdworks/hypertemplates-go/hypermark"
)

// HyperMark is the markdown parser for HyperText documents. Parse populates
// the public fields with the rendered HTML, extracted front matter, mentions,
// and computed metrics.
type HyperMark struct {
	Config      *HyperMarkConfig
	FrontMatter map[string]any
	Content     string
	Summary     string
	More        string
	Mentions    hypermark.Mentions
	WordCount   int
	goldmark    goldmark.Markdown
}

// HyperMarkConfig configures the markdown parser.
type HyperMarkConfig struct {
	Mentions hypermark.MentionConfig `json:"mentions" yaml:"mentions"`
}

// DefaultHyperMarkConfig is the default configuration applied when New
// receives a nil config.
var DefaultHyperMarkConfig = HyperMarkConfig{
	Mentions: hypermark.DefaultMentionConfig,
}

var moreDelimiter = regexp.MustCompile(`<!--\s*more\s*-->`)

// Parse renders the markdown content and populates the receiver's fields.
func (md *HyperMark) Parse(content []byte) error {
	var buf bytes.Buffer
	context := parser.NewContext()
	if err := md.goldmark.Convert(content, &buf, parser.WithContext(context)); err != nil {
		return err
	}
	md.extractFrontMatter(context)
	md.extractMentions(context)
	md.extractWordCount(context)
	md.SummarizeContent(buf.Bytes())
	return nil
}

func (md *HyperMark) extractFrontMatter(context parser.Context) {
	fm := meta.Get(context)
	if fm == nil {
		md.FrontMatter = map[string]any{}
		return
	}
	// goldmark-meta uses yaml.v2, which emits map[any]any for nested maps.
	// Normalize to map[string]any here so callers downstream never see the
	// interface-keyed shape.
	md.FrontMatter = stringifyKeys(fm)
}

func (md *HyperMark) extractMentions(context parser.Context) {
	if mentions, ok := context.Get(hypermark.MentionContext).(hypermark.Mentions); ok {
		md.Mentions = mentions
	}
}

func (md *HyperMark) extractWordCount(context parser.Context) {
	if count, ok := context.Get(hypermark.WordCounterContext).(int); ok {
		md.WordCount = count
	}
}

// SummarizeContent splits htmlBytes on the <!-- more --> delimiter (and its
// whitespace variants), populating Content, Summary, and More on the
// receiver. When the delimiter is absent or malformed, only Content is set.
// Two delimiters split on the first occurrence; subsequent ones are left in
// More verbatim.
func (md *HyperMark) SummarizeContent(htmlBytes []byte) {
	md.Content = string(htmlBytes)
	loc := moreDelimiter.FindIndex(htmlBytes)
	if loc == nil {
		return
	}
	md.Summary = string(htmlBytes[:loc[0]])
	md.More = string(htmlBytes[loc[1]:])
}

// TemplateData returns the markdown's parsed front matter folded together with
// the rendered content, summary, more-content, mentions, and word count.
//
// The shape mirrors the original HyperMark.Data() helper:
//   - Front-matter keys appear at the top level.
//   - "content", "summary", "more" carry the rendered HTML.
//   - "mentions" carries the extracted Mentions map.
//   - "tags" is augmented with discovered hashtag mentions, sorted and deduped.
//   - "wordcount" carries the extracted word count when non-zero.
func (md *HyperMark) TemplateData() data.TemplateData {
	out := data.TemplateData{}
	for k, v := range md.FrontMatter {
		out[k] = v
	}
	out.Set("content", md.Content)
	if len(md.Summary) > 0 {
		out.Set("summary", md.Summary)
	}
	if len(md.More) > 0 {
		out.Set("more", md.More)
	}
	if len(md.Mentions) > 0 {
		out.Set("mentions", md.Mentions)
		applyHashtagMentions(out, md.Mentions, "tags")
	}
	if md.WordCount > 0 {
		out.Set("wordcount", md.WordCount)
	}
	return out
}

// Resolve fills empty mention prefix/suffix fields with their defaults.
func (config *HyperMarkConfig) Resolve() {
	if config == nil {
		return
	}
	if len(config.Mentions.HrefPrefix) == 0 {
		config.Mentions.HrefPrefix = hypermark.DefaultMentionConfig.HrefPrefix
	}
	if len(config.Mentions.HrefSuffix) == 0 {
		config.Mentions.HrefSuffix = hypermark.DefaultMentionConfig.HrefSuffix
	}
}

func (config HyperMarkConfig) JSONSchemaTitle() string {
	return "HyperMark configuration"
}
func (HyperMarkConfig) JSONSchemaName() string    { return "markdown" }
func (HyperMarkConfig) JSONSchemaSummary() string { return "Markdown rendering configuration." }

func (config HyperMarkConfig) JSONSchemaDescription() string {
	return "Markdown rendering configuration for HyperTexting content."
}

// New constructs a HyperMark parser. A nil config is replaced with
// DefaultHyperMarkConfig.
func New(config *HyperMarkConfig) *HyperMark {
	if config == nil {
		config = &DefaultHyperMarkConfig
	}
	md := &HyperMark{
		Config:      config,
		FrontMatter: map[string]any{},
		Mentions:    hypermark.Mentions{},
	}
	md.goldmark = goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
			extension.DefinitionList,
			emoji.Emoji,
			meta.Meta,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.WithLineNumbers(true),
				),
			),
			hypermark.EntityMentionExtension(&md.Config.Mentions),
			hypermark.HashtagMentionExtension(&md.Config.Mentions),
			hypermark.WordCounterExtension(),
			hypermark.HTBlockHTMLParserExtension(),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
	return md
}

// UnmarshalTemplateData parses markdown bytes and returns the rendered output
// folded into a TemplateData. It is the markdown-side equivalent of
// data.UnmarshalJSON / data.UnmarshalYAML.
func UnmarshalTemplateData(b []byte, config *HyperMarkConfig) (data.TemplateData, error) {
	md := New(config)
	if err := md.Parse(b); err != nil {
		return nil, err
	}
	return md.TemplateData(), nil
}

func applyHashtagMentions(d data.TemplateData, mentions hypermark.Mentions, key string) {
	tags, ok := d.Get(key).([]any)
	if !ok {
		tags = []any{}
	}
	for _, mention := range mentions["hashtag"] {
		tags = append(tags, mention)
	}
	d.Set(key, dedupTags(tags))
}

func dedupTags(incoming []any) []any {
	seen := make([]string, 0, len(incoming))
	for _, v := range incoming {
		seen = append(seen, fmt.Sprintf("%v", v))
	}
	slices.Sort(seen)
	seen = slices.Compact(seen)
	out := make([]any, len(seen))
	for i, v := range seen {
		out[i] = v
	}
	return out
}

// stringifyKeys is the markdown-package-local copy of data.stringifyKeys. It
// exists so we don't have to expose the helper from the data package as part
// of its public API.
func stringifyKeys(v any) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = stringifyValue(vv)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[fmt.Sprintf("%v", k)] = stringifyValue(vv)
		}
		return out
	}
	return map[string]any{}
}

func stringifyValue(v any) any {
	switch x := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[fmt.Sprintf("%v", k)] = stringifyValue(vv)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = stringifyValue(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = stringifyValue(vv)
		}
		return out
	}
	return v
}
