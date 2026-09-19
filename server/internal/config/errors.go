package config

import "errors"

var (
	ErrUnavailable   = errors.New("configuration repository unavailable")
	ErrStoredInvalid = errors.New("stored configuration is invalid")
)
