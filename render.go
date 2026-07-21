package hypertemplates

import (
	"io"
	"net/url"
	"os"

	"github.com/herdworks/hypertemplates-go/directives"
	"github.com/herdworks/hypertemplates-go/document"
	"github.com/herdworks/hypertemplates-go/theme"
	"golang.org/x/net/html"
)

func Render(dataPath string, layoutPath string, themePath string, untidy bool, markdownConfig *HyperMarkConfig, w io.Writer) ([]byte, error) {
	if w == nil {
		w = os.Stdout
	}

	var data TemplateData
	var layout *html.Node
	var t *theme.Theme
	var doc *document.Document
	var err error

	if data, err = ParseTemplateDataFile(dataPath, markdownConfig); err != nil {
		return nil, err
	}

	t = &theme.Theme{URI: &url.URL{Path: themePath}}
	if layout, err = t.ResourceHTML(layoutPath); err != nil {
		return nil, err
	}

	doc = &document.Document{
		Theme:          t,
		Layout:         layout,
		TidyMode:       !untidy,
		MarkdownConfig: markdownConfig,
	}
	return directives.Process(doc, &data, nil, w)
}
