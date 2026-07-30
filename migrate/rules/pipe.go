package rules

import (
	migrationtypes "github.com/herdworks/hypertemplates-go/migrate/types"
)

// Pipe reports push-based ht-pipe declarations for manual pull-based migration.
type Pipe struct{}

func (Pipe) Selectors() []string {
	return []string{
		"[data-ht-pipe]:not([data-ht-pipe^='from '])",
		"[ht-pipe]:not([ht-pipe^='from '])",
	}
}

func (Pipe) Proposal(candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	return nil, manualPipeMigration(candidate)
}

func (Pipe) Apply(candidate *migrationtypes.Candidate) error { return manualPipeMigration(candidate) }

func manualPipeMigration(candidate *migrationtypes.Candidate) error {
	if candidate == nil || candidate.Element == nil {
		return &migrationtypes.ManualMigrationRequiredError{Message: "pipe candidate element is nil"}
	}
	return &migrationtypes.ManualMigrationRequiredError{Message: "ht-pipe is now pull-based: annotate the destination with `from \"<source selector>\" [as <type>]` and mark the source elements explicitly"}
}
