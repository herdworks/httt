package variable

import (
	"errors"
	"fmt"
)

// MissingValueError indicates that a variable in the source evaluated to nil
// with no fallback. Callers can detect it with errors.As; the Source field
// reports the complete source that failed.
type MissingValueError struct {
	Source string
}

var (
	errInvalidVariable      = errors.New("invalid variable")
	errUnsupportedComment   = errors.New("comments are not supported in variable expressions")
	errInvalidKeypath       = errors.New("invalid keypath")
	errUnterminatedVariable = errors.New("unterminated variable")
)

func (e *MissingValueError) Error() string {
	return fmt.Sprintf("a template variable in %s didn't return a value", e.Source)
}
