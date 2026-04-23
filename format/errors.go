package format

import (
	"errors"
)

// VARIABLES ===================================================================

var (
	errInvalidTemplate    = errors.New("invalid format template")
	errTemplateComment    = errors.New("template comments are not supported")
	errInvalidKeypath     = errors.New("invalid keypath")
	errUnterminatedAction = errors.New("unterminated template action")
)
