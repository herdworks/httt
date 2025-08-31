package opml

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
)

// OPML Types
// Spec: https://opml.org/spec2.opml
type OPML struct {
	XMLName xml.Name `xml:"opml" json:"opml"`
	Head    Head     `xml:"head" json:"head"`
	Body    Body     `xml:"body" json:"body"`
	Version string   `xml:"version,attr,omitempty" json:"version,omitempty"`
}

type Head struct {
	Title        string `xml:"title,omitempty" json:"title,omitempty"`
	OwnerId      string `xml:"ownerId,omitempty" json:"owner_id,omitempty"`
	OwnerName    string `xml:"ownerName,omitempty" json:"owner_name,omitempty"`
	OwnerEmail   string `xml:"ownerEmail,omitempty" json:"owner_email,omitempty"`
	DateCreated  string `xml:"dateCreated,omitempty" json:"date_created,omitempty"`
	DateModified string `xml:"dateModified,omitempty" json:"date_modified,omitempty"`
}

type Body struct {
	Outlines Outlines `xml:"outline" json:"outlines"`
}

type Outline struct {
	Text         string    `xml:"text,attr" json:"text,omitempty"`
	Type         string    `xml:"type,attr,omitempty" json:"type,omitempty"`
	Title        string    `xml:"title,attr,omitempty" json:"title,omitempty"`
	Description  string    `xml:"description,attr,omitempty" json:"description,omitempty"`
	HTMLURL      string    `xml:"htmlUrl,attr,omitempty" json:"htmlUrl,omitempty"` // e.g. https://hypertemplates.net/
	XMLURL       string    `xml:"xmlUrl,attr,omitempty" json:"xmlUrl,omitempty"`   // e.g. https://hypertemplates.net/atom.xml
	Created      string    `xml:"created,attr,omitempty" json:"created,omitempty"`
	Category     string    `xml:"category,attr,omitempty" json:"category,omitempty"`
	Version      string    `xml:"version,attr,omitempty" json:"version,omitempty"`
	Outlines     []Outline `xml:"outline" json:"outlines,omitempty"`
	IsComment    bool      `xml:"isComment,attr,omitempty" json:"isComment,omitempty"`
	IsBreakpoint bool      `xml:"isBreakpoint,attr,omitempty" json:"isBreakpoint,omitempty"`

	// custom attributes for HyperTemplates
	FaviconURL string `xml:"faviconUrl,attr,omitempty" json:"faviconUrl,omitempty"`
	AudioURL   string `xml:"audioUrl,attr,omitempty" json:"audioUrl,omitempty"` // podcast
	VideoURL   string `xml:"videoUrl,attr,omitempty" json:"videoUrl,omitempty"` // vlog
}
type Outlines []*Outline

// OPML Methods
func (opml *OPML) Add(outline *Outline) {
	opml.Body.Outlines = append(opml.Body.Outlines, outline)
}
func (opml *OPML) AddRSS(outline *Outline) {
	outline.Type = "rss"
	outline.Version = "RSS"
	opml.Add(outline)
}

func (opml *OPML) Render() (string, error) {
	if opml.Version == "" {
		opml.Version = "2.0"
	}
	atomBytes, err := xml.MarshalIndent(opml, "", "\t")
	if err != nil {
		return "", err
	}
	return string(atomBytes), nil
}

func (opml *OPML) Validate() error {
	for _, outline := range opml.Body.Outlines {
		if err := outline.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (opml *OPML) JSON() []byte {
	jsonBytes, err := json.Marshal(opml)
	if err != nil {
		return []byte{}
	}
	return jsonBytes
}

// Outline Methods
func (outline *Outline) Validate() error {
	if outline.Type == "" {
		return fmt.Errorf("outline.type is required")
	}
	if outline.Text == "" {
		return fmt.Errorf("outline.text is required")
	}
	if outline.XMLURL == "" {
		return fmt.Errorf("outline.xmlUrl is required")
	}
	return nil
}
