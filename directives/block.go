package directives

import (
	"io"
	"os"
	"path/filepath"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var HyperBlockAttributeStrings = []string{"data-ht-block", "ht-block"}

func RenderBlock(document *htdocument.Document, element *html.Node, userData *htdata.TemplateData, w io.Writer) (err error) {
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
	if fragment.Type == html.ElementNode && !htdocument.HasAttribute(fragment, HyperApplyAttributeStrings...) {
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
	process(&shortcode, &fragmentData, userData, w)
	if implicitApply {
		htdocument.RemoveAttributes(fragment, "data-ht-apply")
	}
	element.Parent.InsertBefore(shortcode.Layout, element)
	if document.TidyMode {
		htdocument.RemoveElement(element)
	}
	return nil
}

func HyperBlockElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-block]"}
	if document.StrictMode {
		return htdocument.QueryRoot(document.Layout, selectors...)
	}
	selectors = append(selectors, "[ht-block]")
	return htdocument.QueryRoot(document.Layout, selectors...)
}
