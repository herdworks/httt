package types

import (
	"fmt"

	"github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

// Candidate identifies one element selected for migration.
type Candidate struct {
	Element *html.Node
	File    string
}

// Clone returns a detached deep copy suitable for constructing a proposal.
func (candidate *Candidate) Clone() *Candidate {
	if candidate == nil {
		return nil
	}
	var element *html.Node
	if candidate.Element != nil {
		element = document.CloneElement(candidate.Element)
	}
	return &Candidate{
		Element: element,
		File:    candidate.File,
	}
}

// ReplaceElement replaces the candidate's owning element in its current DOM.
func (candidate *Candidate) ReplaceElement(replacement *html.Node) error {
	if candidate == nil || candidate.Element == nil {
		return fmt.Errorf("candidate element is nil")
	}
	if replacement == nil {
		return fmt.Errorf("replacement element is nil")
	}
	if replacement.Parent != nil || replacement.PrevSibling != nil || replacement.NextSibling != nil {
		return fmt.Errorf("replacement element is already attached")
	}
	current := candidate.Element
	if current.Parent != nil {
		parent := current.Parent
		parent.InsertBefore(replacement, current)
		parent.RemoveChild(current)
	}
	candidate.Element = replacement
	return nil
}
