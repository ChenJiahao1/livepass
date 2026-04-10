package rush

const (
	AttemptStateAccepted   = "ACCEPTED"
	AttemptStateProcessing = "PROCESSING"
	AttemptStateSuccess    = "SUCCESS"
	AttemptStateFailed     = "FAILED"
)

const (
	AttemptReasonOrderCommitted        = "ORDER_COMMITTED"
	AttemptReasonUserHoldConflict      = "USER_HOLD_CONFLICT"
	AttemptReasonViewerHoldConflict    = "VIEWER_HOLD_CONFLICT"
	AttemptReasonQuotaExhausted        = "QUOTA_EXHAUSTED"
	AttemptReasonSeatExhausted         = "SEAT_EXHAUSTED"
	AttemptReasonClosedOrderReleased   = "CLOSED_ORDER_RELEASED"
	AttemptReasonAlreadyHasActiveOrder = "ALREADY_HAS_ACTIVE_ORDER"
)

const (
	PollOrderStatusProcessing int64 = 1
	PollOrderStatusVerifying  int64 = 2
	PollOrderStatusSuccess    int64 = 3
	PollOrderStatusFailed     int64 = 4
)

type AttemptTransitionOutcome string

const (
	AttemptTransitioned     AttemptTransitionOutcome = "transitioned"
	AttemptAlreadyFailed    AttemptTransitionOutcome = "already_failed"
	AttemptAlreadySucceeded AttemptTransitionOutcome = "already_succeeded"
	AttemptLostOwnership    AttemptTransitionOutcome = "lost_ownership"
	AttemptStateMissing     AttemptTransitionOutcome = "state_missing"
)

const (
	AdmitDecisionRejected int64 = 0
	AdmitDecisionAccepted int64 = 1
	AdmitDecisionReused   int64 = 2
)

const (
	AdmitRejectNone                   int64 = 0
	AdmitRejectUserInflightConflict   int64 = 1001
	AdmitRejectViewerInflightConflict int64 = 1002
	AdmitRejectQuotaExhausted         int64 = 1003
)
