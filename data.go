package hypertemplates

import (
	"github.com/herdworks/hypertemplates-go/data"
	"github.com/herdworks/hypertemplates-go/htmldata"
	"github.com/herdworks/hypertemplates-go/markdown"
	"github.com/herdworks/hypertemplates-go/opmldata"
	"github.com/herdworks/hypertemplates-go/parse"
)

// TemplateData is re-exported from the data package for backwards
// compatibility. New code should depend on data.TemplateData directly.
type TemplateData = data.TemplateData

// TemplateDataSet is re-exported from the data package for backwards
// compatibility. New code should depend on data.TemplateDataSet directly.
type TemplateDataSet = data.TemplateDataSet

// ParseTemplateDataFile loads a file and parses it into a TemplateData based
// on the file extension. Use parse.ReadFile directly in new code.
func ParseTemplateDataFile(path string, config *HyperMarkConfig) (TemplateData, error) {
	return parse.ReadFile(path, parse.WithMarkdownConfig(config))
}

// ParseTemplateDataBytes parses bytes in the format identified by ext into a
// TemplateData. Use parse.Unmarshal directly in new code.
func ParseTemplateDataBytes(b []byte, ext string, config *HyperMarkConfig) (TemplateData, error) {
	return parse.Unmarshal(b, ext, parse.WithMarkdownConfig(config))
}

// ParseTemplateDataOPML parses OPML bytes into a TemplateData. Use
// opmldata.UnmarshalTemplateData directly in new code.
func ParseTemplateDataOPML(b []byte) (TemplateData, error) {
	return opmldata.UnmarshalTemplateData(b)
}

// ParseTemplateDataMarkdown parses markdown bytes into a TemplateData. Use
// markdown.UnmarshalTemplateData directly in new code.
func ParseTemplateDataMarkdown(b []byte, config *HyperMarkConfig) (TemplateData, error) {
	return markdown.UnmarshalTemplateData(b, config)
}

// ParseTemplateDataHTML parses HTML bytes into a TemplateData. Use
// htmldata.UnmarshalTemplateData directly in new code.
func ParseTemplateDataHTML(b []byte) (TemplateData, error) {
	return htmldata.UnmarshalTemplateData(b)
}
