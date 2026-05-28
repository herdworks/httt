package hypertemplates

import (
	"io"
	"net/url"
	"os"

	"golang.org/x/net/html"
)

func Render(dataPath string, layoutPath string, themePath string, untidy bool, markdownConfig *HyperMarkConfig, w io.Writer) ([]byte, error) {
	if w == nil {
		w = os.Stdout
	}

	var data TemplateData
	var layout *html.Node
	var theme *Theme
	var template *Template
	var err error

	if data, err = ParseTemplateDataFile(dataPath, markdownConfig); err != nil {
		return nil, err
	}

	theme = &Theme{URI: &url.URL{Path: themePath}}
	if layout, err = theme.ResourceHTML(layoutPath); err != nil {
		return nil, err
	}

	template = &Template{
		Theme:          theme,
		Layout:         layout,
		TidyMode:       !untidy,
		MarkdownConfig: markdownConfig,
	}
	return template.Process(&data, nil, w)
}
