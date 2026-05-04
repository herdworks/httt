// Package parse provides a file-extension dispatcher that resolves bytes (or a
// file path) into a TemplateData by selecting the appropriate format-specific
// parser from the data, markdown, htmldata, and opmldata packages.
//
// parse imports every supported format package, so callers that already know
// which format they are reading should depend on the format-specific package
// directly to avoid the larger transitive dependency graph.
package parse

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/herdworks/hypertemplates-go/data"
	"github.com/herdworks/hypertemplates-go/htmldata"
	"github.com/herdworks/hypertemplates-go/markdown"
	"github.com/herdworks/hypertemplates-go/opmldata"
)

// Option configures format-specific behavior on a parse call.
type Option func(*options)

type options struct {
	markdownConfig *markdown.HyperMarkConfig
}

// WithMarkdownConfig overrides the default *markdown.HyperMarkConfig used when
// dispatching to the markdown parser. Has no effect on non-markdown formats.
func WithMarkdownConfig(c *markdown.HyperMarkConfig) Option {
	return func(o *options) { o.markdownConfig = c }
}

// ReadFile reads a file from disk and parses its contents into a TemplateData
// based on the file extension. Empty files return an empty TemplateData.
func ReadFile(path string, opts ...Option) (data.TemplateData, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return data.TemplateData{}, nil
	}
	return Unmarshal(b, filepath.Ext(path), opts...)
}

// Unmarshal parses bytes in the format identified by ext into a TemplateData.
// ext is matched case-insensitively and may include or omit the leading dot:
// ".json", "json", ".YAML" all work.
func Unmarshal(b []byte, ext string, opts ...Option) (data.TemplateData, error) {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	switch normalizeExt(ext) {
	case ".json":
		return data.UnmarshalJSON(b)
	case ".yaml", ".yml":
		return data.UnmarshalYAML(b)
	case ".md", ".markdown":
		return markdown.UnmarshalTemplateData(b, o.markdownConfig)
	case ".opml":
		return opmldata.UnmarshalTemplateData(b)
	case ".html", ".htm":
		return htmldata.UnmarshalTemplateData(b)
	}
	return nil, fmt.Errorf("parse: unsupported file type: %s", ext)
}

func normalizeExt(ext string) string {
	if ext == "" {
		return ""
	}
	if ext[0] != '.' {
		ext = "." + ext
	}
	// lowercase manually to avoid pulling strings just for ToLower
	out := make([]byte, len(ext))
	for i := 0; i < len(ext); i++ {
		c := ext[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}
