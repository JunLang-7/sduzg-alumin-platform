package common

import "errors"

var (
	ErrDatabaseUnavailable = errors.New("database unavailable")
	ErrCacheUnavailable    = errors.New("cache unavailable")

	ErrInvalidRequest       = errors.New("invalid request")
	ErrUserNotFound         = errors.New("user not found")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrCannotDeleteSelf     = errors.New("cannot delete self")
	ErrCannotDeleteSuper    = errors.New("cannot delete super admin")
	ErrAlumniNotFound       = errors.New("alumni not found")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrAlumniProfileUnbound = errors.New("alumni profile unbound")

	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrAccountLocked      = errors.New("account temporarily locked")

	ErrCodeExpired    = errors.New("verification code expired")
	ErrCodeInvalid    = errors.New("invalid verification code")
	ErrCodeConsumed   = errors.New("verification code already used")
	ErrRateLimited    = errors.New("rate limited, please try again later")
	ErrAlumniNotMatch = errors.New("未找到匹配的校友信息")

	ErrFileNotFound       = errors.New("file not found")
	ErrFileTooLarge       = errors.New("file too large")
	ErrFileTypeNotAllowed = errors.New("file type not allowed")
	ErrStorageUnavailable = errors.New("storage unavailable")
)
