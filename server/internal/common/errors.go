package common

import "errors"

var (
	ErrDatabaseUnavailable = errors.New("database unavailable")
	ErrCacheUnavailable    = errors.New("cache unavailable")

	ErrInvalidRequest       = errors.New("invalid request")
	ErrUserNotFound         = errors.New("user not found")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrAlumniNotFound       = errors.New("alumni not found")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrAlumniProfileUnbound = errors.New("alumni profile unbound")

	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account temporarily locked")
)
