package xerr

import "errors"

var (
	ErrInvalidParam = errors.New("invalid param")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInternal     = errors.New("internal error")
)
