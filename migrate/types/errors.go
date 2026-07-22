package types

import "fmt"

// ManualMigrationRequiredError reports that a rule cannot safely propose a migration.
type ManualMigrationRequiredError struct {
	Message string
}

// MigrationWarning reports a successfully applied migration that needs review.
type MigrationWarning struct {
	Message string
}

// AbortMigrationError requests immediate migration termination.
type AbortMigrationError struct {
	Err error
}

func (err *ManualMigrationRequiredError) Error() string {
	if err == nil || err.Message == "" {
		return "manual migration required"
	}
	return err.Message
}

func (warning *MigrationWarning) Error() string {
	if warning == nil || warning.Message == "" {
		return "migration applied with warning"
	}
	return warning.Message
}

func (err *AbortMigrationError) Error() string {
	if err == nil || err.Err == nil {
		return "migration aborted by rule"
	}
	return fmt.Sprintf("migration aborted by rule: %v", err.Err)
}

func (err *AbortMigrationError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}
