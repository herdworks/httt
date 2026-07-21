package document

import (
	"github.com/herdworks/hypertemplates-go/markdown"
	"github.com/herdworks/hypertemplates-go/theme"
	"github.com/herdworks/hypertemplates-go/variable"
	"golang.org/x/net/html"
)

// Document is a renderable HTML document or document fragment with
// HyperTemplates rendering configuration.
type Document struct {
	Layout         *html.Node
	Theme          *theme.Theme
	StrictMode     bool
	TidyMode       bool
	MarkdownConfig *markdown.HyperMarkConfig
	Plugins        variable.Plugins

	// Internal state
	BlockMode bool
}

// IsFragment reports whether the document is rooted at something other than
// a complete HTML DocumentNode.
func (document *Document) IsFragment() bool {
	return document != nil && document.Layout != nil && document.Layout.Type != html.DocumentNode
}

func (document *Document) EnsureMarkdownConfig() {
	if document.MarkdownConfig == nil {
		document.MarkdownConfig = &markdown.DefaultHyperMarkConfig
	}
}
