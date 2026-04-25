package value

import (
	"errors"
)

// VARIABLES ===================================================================

var (
	errInvalidValue       = errors.New("invalid template value")
	errTemplateComment    = errors.New("template comments are not supported")
	errInvalidKeypath     = errors.New("invalid keypath")
	errUnterminatedAction = errors.New("unterminated template action")
)
