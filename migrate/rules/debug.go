package rules

import (
	migrationtypes "github.com/herdworks/httt/migrate/types"
)

// Debug reports legacy data-mode ht-debug declarations for manual migration.
type Debug struct{}

func (Debug) Selectors() []string {
	return []string{`[ht-debug$='data']`, `[data-ht-debug$='data']`}
}

func (Debug) Proposal(*migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	return nil, legacyDebugDataError()
}

func (Debug) Apply(*migrationtypes.Candidate) error {
	return legacyDebugDataError()
}

func legacyDebugDataError() error {
	return &migrationtypes.ManualMigrationRequiredError{Message: `ht-debug values ending in "data" must be converted to repeated "element" or ${ ... } attributes`}
}
