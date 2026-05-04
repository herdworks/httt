package hypertemplates

import (
	"github.com/herdworks/hypertemplates-go/markdown"
)

// HyperMark is re-exported from the markdown package for backwards
// compatibility. New code should depend on markdown.HyperMark directly.
type HyperMark = markdown.HyperMark

// HyperMarkConfig is re-exported from the markdown package for backwards
// compatibility. New code should depend on markdown.HyperMarkConfig directly.
type HyperMarkConfig = markdown.HyperMarkConfig

// DefaultHyperMarkConfig is re-exported from the markdown package for
// backwards compatibility. New code should depend on
// markdown.DefaultHyperMarkConfig directly.
//
// Note: this var holds a copy of the markdown package's default; mutating it
// does not affect markdown.DefaultHyperMarkConfig and vice versa. Treat the
// default as read-only.
var DefaultHyperMarkConfig = markdown.DefaultHyperMarkConfig
