// Package opmldata extracts template data from OPML documents.
//
// opmldata is a thin bridge between hyperfeed-go's OPML parser and the
// HyperTemplates data package. The result mirrors the JSON-tagged
// opml.OPML struct shape; the namespace under which this data is
// installed is determined by the caller (typically by the loader's
// filename-based naming rules), not by anything inside the document.
package opmldata

import (
	"encoding/json"

	"github.com/herdworks/hyperfeed-go/types/opml"
	"github.com/herdworks/hypertemplates-go/data"
)

// UnmarshalTemplateData parses OPML bytes and projects the result into a
// TemplateData whose shape mirrors the JSON-tagged opml.OPML struct.
func UnmarshalTemplateData(b []byte) (data.TemplateData, error) {
	doc, err := opml.Parse(b)
	if err != nil {
		return nil, err
	}
	return FromOPML(doc), nil
}

// FromOPML projects an already-parsed *opml.OPML into a TemplateData using
// the same shape as UnmarshalTemplateData.
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

	return d
}
