package hypermark

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Global variables for the HyperMention extension
var (
	EntityMentionTrigger []byte            = []byte{'@'}
	EntityMentionPattern *regexp.Regexp    = regexp.MustCompile(`@([a-zA-Z0-9/_\-]+\.[a-zA-Z0-9/_\-.]+)`)
	EntityMentions       *MentionExtension = &MentionExtension{Trigger: EntityMentionTrigger, Pattern: EntityMentionPattern, External: false}

	HashtagMentionTrigger []byte            = []byte{'#'}
	HashtagMentionPattern *regexp.Regexp    = regexp.MustCompile(`#([a-zA-Z0-9/_-]*[a-zA-Z/_-][a-zA-Z0-9/_-]*)`)
	HashtagMentions       *MentionExtension = &MentionExtension{Trigger: HashtagMentionTrigger, Pattern: HashtagMentionPattern}

	MentionKind    ast.NodeKind      = ast.NewNodeKind("Mention")
	MentionContext parser.ContextKey = parser.NewContextKey() // TODO: use separate contexts for entity mentions and hashtag mentions?
	// _                  goldmark.Extender      = (*MentionExtension)(nil)
	// _                  parser.InlineParser    = (*MentionParser)(nil)
)

func EntityMentionExtension(config *MentionConfig) *MentionExtension {
	if config == nil {
		config = &DefaultMentionConfig
	}
	extension := EntityMentions
	extension.Config = config
	return extension
}
func HashtagMentionExtension(config *MentionConfig) *MentionExtension {
	if config == nil {
		config = &DefaultMentionConfig
	}
	extension := HashtagMentions
	extension.Config = config
	return extension
}

type Mentions map[string][]string

// goldmark.Extender implementation
type MentionExtension struct {
	Config   *MentionConfig
	Trigger  []byte
	Pattern  *regexp.Regexp
	External bool
	Mentions Mentions
}

func (e *MentionExtension) Extend(m goldmark.Markdown) {
	if e.Mentions == nil {
		e.Mentions = Mentions{}
	}
	m.Parser().AddOptions(
		parser.WithInlineParsers(
			util.Prioritized(&MentionParser{
				config:   e.Config,
				trigger:  e.Trigger,
				Pattern:  e.Pattern,
				External: e.External,
				Mentions: &e.Mentions,
			}, 999),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&MentionNodeRenderer{}, 999),
		),
	)
}

// ast.Node implementation
type MentionNode struct {
	ast.BaseInline
	Value    []byte
	Href     []byte
	External bool
}

func (*MentionNode) Kind() ast.NodeKind { return MentionKind }
func (n *MentionNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Value": string(n.Value)}, nil)
}

// parser.InlineParser implementation
type MentionParser struct {
	config   *MentionConfig
	trigger  []byte
	Pattern  *regexp.Regexp
	External bool
	Mentions *Mentions
}

func (p *MentionParser) Trigger() []byte { return p.trigger }
func (p *MentionParser) Kind(node ast.Node) string {
	if p.Trigger()[0] == EntityMentionTrigger[0] {
		return "entity"
	}
	if p.Trigger()[0] == HashtagMentionTrigger[0] {
		return "hashtag"
	}
	fmt.Printf("unknown mention kind: %s\n", p.Trigger())
	return "unknown"
}
func (p *MentionParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, segment := block.PeekLine()
	match := p.Pattern.FindSubmatchIndex(line)
	if match == nil {
		return nil
	}
	mentionByteCount := match[1]
	value := line[1:mentionByteCount]
	for value[len(value)-1] == '.' {
		value = bytes.TrimSuffix(value, []byte{'.'})
		mentionByteCount--
	}
	seg := segment.WithStop(segment.Start + mentionByteCount)
	mention := MentionNode{Value: value, Href: p.Resolve(value), External: p.External}
	mention.AppendChild(&mention, ast.NewTextSegment(seg))
	block.Advance(seg.Len())

	if pc.Get(MentionContext) == nil {
		pc.Set(MentionContext, Mentions{})
	}
	mentions := pc.Get(MentionContext).(Mentions)
	mentions[p.Kind(&mention)] = append(mentions[p.Kind(&mention)], string(value))
	pc.Set(MentionContext, mentions)

	return &mention
}
func (p *MentionParser) Resolve(value []byte) []byte {
	var href []byte
	switch p.Trigger()[0] {
	case EntityMentionTrigger[0]:
		href = fmt.Appendf(href, "https://%s", value)
	case HashtagMentionTrigger[0]:
		href = fmt.Append(href, p.config.TagPrefix(), string(value), p.config.TagSuffix())
	default:
		href = nil
	}
	if href != nil {
		href = bytes.ToLower(href)
	}
	return href
}

// renderer.NodeRenderer implementation
type MentionNodeRenderer struct{}

func (r *MentionNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(MentionKind, r.Render)
}
func (r *MentionNodeRenderer) Render(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*MentionNode)
	if !ok {
		return ast.WalkStop, fmt.Errorf("unexpected node %T, expected *Node", node)
	}
	if entering {
		if err := r.enter(w, n); err != nil {
			return ast.WalkStop, err
		}
	} else {
		r.exit(w, n)
	}
	return ast.WalkContinue, nil
}
func (r *MentionNodeRenderer) enter(w util.BufWriter, n *MentionNode) error {
	_, _ = w.WriteString(`<a`)
	_, _ = w.WriteString(` href="`)
	_, _ = w.Write(util.URLEscape(n.Href, true /* resolve references */))
	_, _ = w.WriteString(`"`)
	_, _ = w.WriteString(` data-hyper-mention`)
	if n.External {
		_, _ = w.WriteString(` target='_blank'`)
	}
	_, _ = w.WriteString(`>`)
	return nil
}
func (r *MentionNodeRenderer) exit(w util.BufWriter, _ *MentionNode) {
	_, _ = w.WriteString("</a>")
}

type MentionConfig struct {
	HrefPrefix string `json:"href_prefix" yaml:"href_prefix"`
	HrefSuffix string `json:"href_suffix" yaml:"href_suffix"`
}

var DefaultMentionConfig = MentionConfig{
	HrefPrefix: "/tags/",
	HrefSuffix: "/",
}

func (m *MentionConfig) TagPrefix() string {
	if m == nil {
		m = &DefaultMentionConfig
	}
	if len(m.HrefPrefix) == 0 {
		m.HrefPrefix = DefaultMentionConfig.HrefPrefix
	}
	return m.HrefPrefix
}
func (m *MentionConfig) TagSuffix() string {
	if m == nil {
		m = &DefaultMentionConfig
	}
	if len(m.HrefSuffix) == 0 {
		m.HrefSuffix = DefaultMentionConfig.HrefSuffix
	}
	return m.HrefSuffix
}
