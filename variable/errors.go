package variable

import (
	"errors"
	"fmt"
)

// VARIABLES ===================================================================

var (
	errInvalidValue       = errors.New("invalid template value")
	errTemplateComment    = errors.New("template comments are not supported")
	errInvalidKeypath     = errors.New("invalid keypath")
	errUnterminatedAction = errors.New("unterminated template action")
)

// TYPES =======================================================================

// MissingValueError indicates that a template source produced a nil action
// value with no fallback. Callers can detect it with errors.As; the Source
// field reports the template source that failed.
type MissingValueError struct {
	Source string
}

func (e *MissingValueError) Error() string {
	return fmt.Sprintf("a template variable in %s didn't return a value", e.Source)
}
