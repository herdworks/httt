package hypertemplates

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	hypermark "github.com/herdworks/hypertemplates-go/hypermark"
	emoji "github.com/yuin/goldmark-emoji"
	meta "github.com/yuin/goldmark-meta"
)

// TYPES =======================================================================
// HyperMark is the markdown parser for HyperText documents.
type HyperMark struct {
	Config      *HyperMarkConfig
	FrontMatter TemplateData
	Content     string
	Summary     string
	More        string
	Mentions    hypermark.Mentions
	WordCount   int
	goldmark    goldmark.Markdown
}

type HyperMarkConfig struct {
	Mentions hypermark.MentionConfig `json:"mentions" yaml:"mentions"`
}

// VARIABLES ===================================================================
var (
	moreDelimiter = regexp.MustCompile(`<!--\s*more\s*-->`)
)

// METHODS =====================================================================
func (md HyperMark) New(config *HyperMarkConfig) *HyperMark {
	if config == nil {
		config = &DefaultHyperMarkConfig
	}
	md = HyperMark{}
	md.Config = config
	md.Content = ""
	md.FrontMatter = TemplateData{}
	md.Mentions = hypermark.Mentions{}
	md.WordCount = 0
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
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(
			html.WithXHTML(),
			html.WithUnsafe(),
		),
	)
	return &md
}

func (md *HyperMark) Parse(content []byte) error {
	var buf bytes.Buffer
	context := parser.NewContext()
	err := md.goldmark.Convert(content, &buf, parser.WithContext(context))
	if err != nil {
		return err
	}
	md.ExtractFrontmatter(context)
	md.ExtractMentions(context)
	md.ExtractWordCount(context)
	md.SummarizeContent(buf.Bytes())
	return nil
}

func (md *HyperMark) ExtractFrontmatter(context parser.Context) {
	if fm := meta.Get(context); fm != nil {
		md.FrontMatter = fm
	}
	md.FrontMatter.StringifyMapKeys() // workaround for https://github.com/go-yaml/yaml/issues/139 (because goldmark-meta uses yaml.v2)
}

func (md *HyperMark) ExtractMentions(context parser.Context) {
	mentions, ok := context.Get(hypermark.MentionContext).(hypermark.Mentions)
	if ok {
		md.Mentions = mentions
	}
}
func (md *HyperMark) ExtractWordCount(context parser.Context) {
	count, ok := context.Get(hypermark.WordCounterContext).(int)
	if ok {
		md.WordCount = count
	}
}

func (md *HyperMark) SummarizeContent(htmlBytes []byte) {
	md.Content = string(htmlBytes)
	loc := moreDelimiter.FindIndex(htmlBytes)
	if loc == nil {
		return // no <!--more--> delimiter found
	}
	md.Summary = string(htmlBytes[:loc[0]])
	md.More = string(htmlBytes[loc[1]:])
}

func (md *HyperMark) Data() (data TemplateData) {
	data = md.FrontMatter
	if data == nil {
		data = TemplateData{}
	}
	data.Set("content", md.Content)
	if len(md.Summary) > 0 {
		data.Set("summary", md.Summary)
	}
	if len(md.More) > 0 {
		data.Set("more", md.More)
	}
	if len(md.Mentions) > 0 {
		data.Set("mentions", md.Mentions)
		md.ApplyMentions(&data, "tags")
	}
	if md.WordCount > 0 {
		data.Set("wordcount", md.WordCount)
	}
	return data
}

func (md *HyperMark) ApplyMentions(data *TemplateData, tagpath string) {
	tags, ok := data.Get(tagpath).([]any)
	if !ok {
		tags = []any{}
	}
	for _, mention := range md.Mentions["hashtag"] {
		tags = append(tags, mention)
	}
	tags = md.DedupTags(tags)
	data.Set(tagpath, tags)
}

func (md *HyperMark) DedupTags(incoming []any) []any {
	tags := []string{}
	for _, v := range incoming {
		tags = append(tags, fmt.Sprintf("%v", v))
	}
	slices.Sort(tags)
	tags = slices.Compact(tags)
	outgoing := []any{}
	for _, tag := range tags {
		outgoing = append(outgoing, tag)
	}
	return outgoing
}

func (config *HyperMarkConfig) Resolve() {
	if config == nil {
		config = &DefaultHyperMarkConfig
	}
	if len(config.Mentions.HrefPrefix) == 0 {
		config.Mentions.HrefPrefix = hypermark.DefaultMentionConfig.HrefPrefix
	}
	if len(config.Mentions.HrefSuffix) == 0 {
		config.Mentions.HrefSuffix = hypermark.DefaultMentionConfig.HrefSuffix
	}
}

// FUNCTIONS ===================================================================

var DefaultHyperMarkConfig = HyperMarkConfig{
	Mentions: hypermark.DefaultMentionConfig,
}
