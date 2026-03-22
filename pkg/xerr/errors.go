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
	ErrSeatFreezeStatusInvalid       = errors.New("seat freeze status invalid")
	ErrProgramSeatLedgerNotReady     = errors.New("program seat ledger not ready")

	ErrOrderNotFound              = errors.New("order not found")
	ErrOrderExpired               = errors.New("order expired")
	ErrOrderAlreadyPaid           = errors.New("order already paid")
	ErrOrderStatusInvalid         = errors.New("order status invalid")
	ErrOrderRefundNotAllowed      = errors.New("order refund not allowed")
	ErrOrderRefundWindowClosed    = errors.New("order refund window closed")
	ErrOrderRefundRuleInvalid     = errors.New("order refund rule invalid")
	ErrOrderTicketUserInvalid     = errors.New("order ticket user invalid")
	ErrOrderPurchaseLimitExceeded = errors.New("order purchase limit exceeded")
	ErrOrderLimitLedgerNotReady   = errors.New("order limit ledger not ready")
	ErrOrderSubmitTooFrequent     = errors.New("提交频繁，请稍后重试")

	ErrPayBillNotFound = errors.New("pay bill not found")
)
