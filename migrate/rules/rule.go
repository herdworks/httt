package rules

import (
	"fmt"

	"github.com/andybalholm/cascadia"
	"github.com/herdworks/httt/migrate/types"
)

// Rule describes one stateless migration over elements matching its CSS
// selectors. Proposal receives a detached candidate clone; Apply receives the
// live candidate after acceptance.
type Rule interface {
	Selectors() []string
	Proposal(*types.Candidate) (*types.Proposal, error)
	Apply(*types.Candidate) error
}

// Validate verifies that a rule provides valid CSS selectors.
func Validate(rule Rule) error {
	if rule == nil {
		return fmt.Errorf("migration rule is nil")
	}
	selectors := rule.Selectors()
	if len(selectors) == 0 {
		return fmt.Errorf("migration rule %T has no selectors", rule)
	}
	for _, selector := range selectors {
		if selector == "" {
			return fmt.Errorf("migration rule %T has an empty selector", rule)
		}
		if _, err := cascadia.Parse(selector); err != nil {
			return fmt.Errorf("invalid migration selector %q for %T: %w", selector, rule, err)
		}
	}
	return nil
}
