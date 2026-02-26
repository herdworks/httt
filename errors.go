package hypertemplates

import "errors"

var (
	ErrInclude         = errors.New("ht-include error")
	ErrIncludeFragment = errors.New("ht-include fragment error")
)
