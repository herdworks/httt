package rules

import (
	"errors"
	"strings"

	"github.com/herdworks/hypertemplates-go/document"
	migrationtypes "github.com/herdworks/hypertemplates-go/migrate/types"
)

// Debug removes optional ht-debug directives when explicitly accepted.
type Debug struct{}

func (Debug) Selectors() []string {
	return []string{"[ht-debug]", "[data-ht-debug]"}
}

func (Debug) Proposal(candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	name := debugDirectiveName(candidate)
	if err := applyDebug(candidate); err != nil {
		return nil, err
	}
	return &migrationtypes.Proposal{
		After:     candidate.Element,
		Message:   "remove optional " + name + " directive?",
		AutoApply: false,
		Optional:  true,
	}, nil
}

func (Debug) Apply(candidate *migrationtypes.Candidate) error {
	return applyDebug(candidate)
}

func applyDebug(candidate *migrationtypes.Candidate) error {
	if candidate == nil || candidate.Element == nil {
		return errors.New("debug candidate element is nil")
	}
	document.RemoveAttributes(candidate.Element, "ht-debug", "data-ht-debug")
	return nil
}

func debugDirectiveName(candidate *migrationtypes.Candidate) string {
	for _, attribute := range candidate.Element.Attr {
		switch strings.ToLower(attribute.Key) {
		case "ht-debug", "data-ht-debug":
			return attribute.Key
		}
	}
	return "ht-debug"
}
