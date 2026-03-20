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

	ErrProgramShowTimeNotFound       = errors.New("program show time not found")
	ErrProgramTicketCategoryNotFound = errors.New("program ticket category not found")
	ErrSeatInventoryInsufficient     = errors.New("seat inventory insufficient")
	ErrSeatFreezeNotFound            = errors.New("seat freeze not found")
	ErrSeatFreezeRequestConflict     = errors.New("seat freeze request conflict")
)
