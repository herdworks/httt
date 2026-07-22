// Package types contains shared migration values, reports, and errors.
package types

import (
	"golang.org/x/net/html"
)

// Proposal describes a concrete candidate mutation for review.
type Proposal struct {
	After     *html.Node
	Message   string
	AutoApply bool
	Optional  bool
}
