package xerr

import "errors"

var (
	ErrInvalidParam = errors.New("invalid param")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInternal     = errors.New("internal error")

	ErrChannelNotFound    = errors.New("channel not found")
	ErrLoginFailedTooMany = errors.New("login failed too many times")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrMobileAlreadyUsed  = errors.New("mobile already used")
	ErrEmailAlreadyUsed   = errors.New("email already used")
	ErrTicketUserExists   = errors.New("ticket user already exists")
)
