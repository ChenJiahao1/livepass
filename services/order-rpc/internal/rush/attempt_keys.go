package rush

import "fmt"

const (
	defaultAttemptPrefix = "livepass:order:rush"

	attemptFieldOrderNumber       = "order_number"
	attemptFieldUserID            = "user_id"
	attemptFieldProgramID         = "program_id"
	attemptFieldShowTimeID        = "show_time_id"
	attemptFieldTicketCategoryID  = "ticket_category_id"
	attemptFieldViewerIDs         = "viewer_ids"
	attemptFieldTicketCount       = "ticket_count"
	attemptFieldSaleWindowEndAt   = "sale_window_end_at"
	attemptFieldShowEndAt         = "show_end_at"
	attemptFieldTokenFingerprint  = "token_fingerprint"
	attemptFieldState             = "state"
	attemptFieldReasonCode        = "reason_code"
	attemptFieldAcceptedAt        = "accepted_at"
	attemptFieldFinishedAt        = "finished_at"
	attemptFieldPublishAttempts   = "publish_attempts"
	attemptFieldProcessingEpoch   = "processing_epoch"
	attemptFieldProcessingStartAt = "processing_started_at"
	attemptFieldCreatedAt         = "created_at"
	attemptFieldTransitionAt      = "last_transition_at"
)

func attemptRecordKey(prefix string, showTimeID, orderNumber int64) string {
	return fmt.Sprintf("%s:%s:attempt:%d", prefix, rushScopeTag(showTimeID), orderNumber)
}

func userInflightKey(prefix string, showTimeID, userID int64) string {
	return fmt.Sprintf("%s:%s:user_inflight:%d", prefix, rushScopeTag(showTimeID), userID)
}

func viewerInflightKey(prefix string, showTimeID, viewerID int64) string {
	return fmt.Sprintf("%s:%s:viewer_inflight:%d", prefix, rushScopeTag(showTimeID), viewerID)
}

func userActiveKey(prefix string, showTimeID, userID int64) string {
	return fmt.Sprintf("%s:%s:user_active:%d", prefix, rushScopeTag(showTimeID), userID)
}

func viewerActiveKey(prefix string, showTimeID, viewerID int64) string {
	return fmt.Sprintf("%s:%s:viewer_active:%d", prefix, rushScopeTag(showTimeID), viewerID)
}

func quotaAvailableKey(prefix string, showTimeID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:%s:quota:%d", prefix, rushScopeTag(showTimeID), ticketCategoryID)
}

func seatOccupiedKey(prefix string, showTimeID, orderNumber int64) string {
	return fmt.Sprintf("%s:%s:seat_occupied:%d", prefix, rushScopeTag(showTimeID), orderNumber)
}

func userFingerprintIndexKey(prefix string, showTimeID, userID int64) string {
	return fmt.Sprintf("%s:%s:fingerprint:%d", prefix, rushScopeTag(showTimeID), userID)
}

func rushScopeTag(showTimeID int64) string {
	return fmt.Sprintf("{st:%d}", showTimeID)
}
