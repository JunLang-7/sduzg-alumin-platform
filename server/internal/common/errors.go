package common

import "errors"

var (
	ErrDatabaseUnavailable = errors.New("database unavailable")
	ErrCacheUnavailable    = errors.New("cache unavailable")

	ErrUserNotFound   = errors.New("user not found")
	ErrAlumniNotFound = errors.New("alumni not found")

	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account temporarily locked")
)
