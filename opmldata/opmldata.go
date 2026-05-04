// Package opmldata extracts template data from OPML documents.
//
// opmldata is a thin bridge between hyperfeed-go's OPML parser and the
// HyperTemplates data package. It owns the project-specific conventions for
// projecting an *opml.OPML into a TemplateData, including the top-level
// "namespace" key derived from the document's Head.Title.
package opmldata

import (
	"encoding/json"
	"strings"

	"github.com/herdworks/hyperfeed-go/types/opml"
	"github.com/herdworks/hypertemplates-go/data"
)

// UnmarshalTemplateData parses OPML bytes and projects the result into a
// TemplateData. The resulting map's shape mirrors the JSON-tagged opml.OPML
// struct, with an additional top-level "namespace" key derived from
// Head.Title (lowercased, spaces and dots stripped). When the title is
// empty the namespace defaults to "opml".
func UnmarshalTemplateData(b []byte) (data.TemplateData, error) {
	doc, err := opml.ParseOPML(b)
	if err != nil {
		return nil, err
	}
	return FromOPML(doc), nil
}

// FromOPML projects an already-parsed *opml.OPML into a TemplateData using
// the same shape and "namespace" rule as UnmarshalTemplateData.
func FromOPML(o *opml.OPML) data.TemplateData {
	if o == nil {
		return data.TemplateData{}
	}

	// Round-trip through JSON to preserve the shape produced by the JSON tags
	// on the opml types. This matches the legacy behavior of
	// ParseTemplateDataOPML and keeps downstream consumers unchanged.
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		// json.Marshal of a well-formed *opml.OPML cannot fail in practice,
		// but degrade to an empty TemplateData rather than panic.
		return data.TemplateData{}
	}
	d := data.TemplateData{}
	if err := json.Unmarshal(jsonBytes, &d); err != nil {
		return data.TemplateData{}
	}

	d["namespace"] = namespace(o.Head.Title)
	return d
}

func namespace(title string) string {
	ns := title
	if ns == "" {
		ns = "opml"
	}
	ns = strings.ReplaceAll(ns, " ", "")
	ns = strings.ReplaceAll(ns, ".", "")
	return strings.ToLower(ns)
}
