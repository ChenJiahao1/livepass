package rush

const (
	AttemptStatePendingPublish = "PENDING_PUBLISH"
	AttemptStateQueued         = "QUEUED"
	AttemptStateProcessing     = "PROCESSING"
	AttemptStateVerifying      = "VERIFYING"
	AttemptStateCommitted      = "COMMITTED"
	AttemptStateReleased       = "RELEASED"
)

const (
	AttemptReasonOrderCommitted      = "ORDER_COMMITTED"
	AttemptReasonUserHoldConflict    = "USER_HOLD_CONFLICT"
	AttemptReasonViewerHoldConflict  = "VIEWER_HOLD_CONFLICT"
	AttemptReasonQuotaExhausted      = "QUOTA_EXHAUSTED"
	AttemptReasonSeatExhausted       = "SEAT_EXHAUSTED"
	AttemptReasonCommitCutoffExceed  = "COMMIT_CUTOFF_EXCEEDED"
	AttemptReasonClosedOrderReleased = "CLOSED_ORDER_RELEASED"
)

const (
	PollOrderStatusProcessing int64 = 1
	PollOrderStatusVerifying  int64 = 2
	PollOrderStatusSuccess    int64 = 3
	PollOrderStatusFailed     int64 = 4
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
