// Package htmldata extracts template data from HTML documents.
//
// HTML documents commonly carry document-level metadata (title, meta
// descriptions, timestamps) that can seed a TemplateData object. htmldata
// covers the small set of conventional extractions used by HyperTemplates
// without dragging the full template engine into callers.
package htmldata

import (
	"bytes"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/herdworks/httt/data"
)

// UnmarshalTemplateData parses HTML bytes and extracts a TemplateData with
// title, description, created_at, and updated_at when present.
func UnmarshalTemplateData(b []byte) (data.TemplateData, error) {
	doc, err := html.Parse(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return ExtractTemplateData(doc), nil
}

// ExtractTemplateData walks an *html.Node tree and returns a TemplateData
// populated with the conventional document-level metadata fields:
//
//   - "title"        ← <title> text content
//   - "description"  ← <meta name="description" content="...">
//   - "created_at"   ← <meta name="created_at"  content="...">
//   - "updated_at"   ← <meta name="updated_at"  content="...">
//
// Missing fields are simply omitted from the returned map. ExtractTemplateData
// always returns a non-nil map.
func ExtractTemplateData(node *html.Node) data.TemplateData {
	d := data.TemplateData{}
	if node == nil {
		return d
	}
	if title := query(node, "title"); title != nil && title.FirstChild != nil {
		d["title"] = title.FirstChild.Data
	}
	if description := query(node, "meta[name=description]"); description != nil {
		d["description"] = attr(description, "content")
	}
	if createdAt := query(node, "meta[name=created_at]"); createdAt != nil {
		d["created_at"] = attr(createdAt, "content")
	}
	if updatedAt := query(node, "meta[name=updated_at]"); updatedAt != nil {
		d["updated_at"] = attr(updatedAt, "content")
	}
	return d
}

// ExtractTemplateDataNodes runs ExtractTemplateData over each node in nodes
// and merges the results in order, with later nodes overwriting earlier ones
// at conflicting top-level keys.
func ExtractTemplateDataNodes(nodes []*html.Node) data.TemplateData {
	out := data.TemplateData{}
	for _, n := range nodes {
		for k, v := range ExtractTemplateData(n) {
			out[k] = v
		}
	}
	return out
}

func query(node *html.Node, selector string) *html.Node {
	sel, err := cascadia.Parse(selector)
	if err != nil {
		return nil
	}
	return cascadia.Query(node, sel)
}

func attr(node *html.Node, key string) string {
	if node == nil || node.Type != html.ElementNode {
		return ""
	}
	for _, a := range node.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
