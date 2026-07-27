package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/herdworks/hypertemplates-go/document"
	migrationtypes "github.com/herdworks/hypertemplates-go/migrate/types"
	"golang.org/x/net/html"
)

// Pipe migrates legacy ht-pipe declarations to the quoted destination syntax.
type Pipe struct{}

func (Pipe) Selectors() []string {
	return []string{
		"[data-ht-pipe]:not([data-ht-pipe^='to '])",
		"[ht-pipe]:not([ht-pipe^='to '])",
	}
}

func (Pipe) Proposal(candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	if err := migratePipe(candidate); err != nil {
		return nil, err
	}
	return &migrationtypes.Proposal{After: candidate.Element, Message: "migrated legacy ht-pipe to quoted destination syntax", AutoApply: true}, nil
}

func (Pipe) Apply(candidate *migrationtypes.Candidate) error { return migratePipe(candidate) }

func migratePipe(candidate *migrationtypes.Candidate) error {
	if candidate == nil || candidate.Element == nil {
		return &migrationtypes.ManualMigrationRequiredError{Message: "legacy pipe candidate element is nil"}
	}
	attributes := document.GetAttributes(candidate.Element, "ht-pipe", "data-ht-pipe")
	if len(attributes) != 1 {
		return &migrationtypes.ManualMigrationRequiredError{Message: "element contains multiple legacy pipe directives"}
	}
	source := attributes[0]
	raw := strings.TrimSpace(source.Val)
	if raw == "" {
		return &migrationtypes.ManualMigrationRequiredError{Message: "legacy pipe declaration is empty"}
	}
	if strings.HasPrefix(raw, "to ") {
		return nil
	}
	if strings.Contains(raw, ";") {
		return &migrationtypes.ManualMigrationRequiredError{Message: "legacy pipe contains multiple declarations"}
	}

	selector := raw
	pipeType := ""
	if legacyType, remainder, found := strings.Cut(raw, ":"); found {
		pipeType = strings.TrimSpace(legacyType)
		selector = strings.TrimSpace(remainder)
		if pipeType == "" || selector == "" {
			return &migrationtypes.ManualMigrationRequiredError{Message: "legacy pipe type declaration is malformed"}
		}
	}
	if selector == "" {
		return &migrationtypes.ManualMigrationRequiredError{Message: "legacy pipe selector is empty"}
	}
	value := "to " + strconv.Quote(selector)
	if pipeType != "" {
		value += " as " + pipeType
	}
	for index := range candidate.Element.Attr {
		if strings.EqualFold(candidate.Element.Attr[index].Key, source.Key) {
			candidate.Element.Attr[index] = html.Attribute{Key: source.Key, Val: value}
			return nil
		}
	}
	return fmt.Errorf("legacy pipe attribute disappeared before migration")
}
