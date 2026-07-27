package directives

import (
	"io"
	"os"
	"path/filepath"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

type BlockDirective struct{}

var _ Directive = BlockDirective{} // make the compiler check Directive interface conformance

func (d BlockDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-block"}
	if !strict {
		attributes = append(attributes, "ht-block")
	}
	return attributes
}

func (d BlockDirective) Selectors(strict bool) []string {
	selectors := []string{"[data-ht-block]"}
	if !strict {
		selectors = append(selectors, "[ht-block]")
	}
	return selectors
}

func (d BlockDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil || document.IsFragment() {
		// only process block directives once, in the root DocumentNode processing loop
		return nil
	}
	return htdocument.QueryRoot(document.Layout, d.Selectors(document.StrictMode)...)
}

func (BlockDirective) NextElements(*htdocument.Document) []*html.Node { return nil }

func (d BlockDirective) Process(document *htdocument.Document, element *html.Node, _ *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) (err error) {
	if document == nil || document.IsFragment() || element == nil {
		return nil
	}
	if w == nil {
		w = os.Stdout
	}

	var fragment *html.Node
	var fragmentRef string = filepath.Join(document.Theme.BlocksDirName(), element.Data)
	var fragmentData htdata.TemplateData
	var fragmentNodes []*html.Node
	if userData != nil {
		fragmentData = userData.Clone()
	} else {
		fragmentData = htdata.TemplateData{}
	}
	if fragment, err = IncludeFragment(document, element, fragmentRef); err != nil {
		return err
	}
	htdocument.ForwardAttributes(element, []*html.Node{fragment}, forwardedAttributeSkipKeys()...)
	fragmentData.Set("block", document.AttributeData(element))
	fragmentNodes = htdocument.ChildNodes(element)
	if len(fragmentNodes) > 0 {
		fragmentData.Set("block.content", string(document.HTML(fragmentNodes...)))
	}

	var implicitApply bool
	if fragment.Type == html.ElementNode && !htdocument.HasAttribute(fragment, ApplyDirective{}.Attributes(document.StrictMode)...) {
		implicitApply = true
		htdocument.SetAttribute(fragment, "data-ht-apply", "")
	}

	shortcode := htdocument.Document{
		Layout:         fragment,
		Theme:          document.Theme,
		TidyMode:       document.TidyMode,
		MarkdownConfig: document.MarkdownConfig,
		Plugins:        document.Plugins,
		BlockMode:      true,
	}
	if _, err = Process(&shortcode, &fragmentData, userData, w); err != nil {
		return err
	}
	if implicitApply {
		htdocument.RemoveAttributes(fragment, "data-ht-apply")
	}
	element.Parent.InsertBefore(shortcode.Layout, element)
	if document.TidyMode {
		htdocument.RemoveElement(element)
	}
	return nil
}
