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
	attemptFieldState             = "state"
	attemptFieldReasonCode        = "reason_code"
	attemptFieldAcceptedAt        = "accepted_at"
	attemptFieldFinishedAt        = "finished_at"
	attemptFieldProcessingStartAt = "processing_started_at"
	attemptFieldCreatedAt         = "created_at"
	attemptFieldTransitionAt      = "last_transition_at"
)

func attemptRecordKey(prefix string, showTimeID, orderNumber int64) string {
	return fmt.Sprintf("%s:attempt:%s:%d", prefix, rushScopeTag(showTimeID), orderNumber)
}

func userInflightKey(prefix string, showTimeID int64) string {
	return fmt.Sprintf("%s:user_inflight:%s", prefix, rushScopeTag(showTimeID))
}

func viewerInflightKey(prefix string, showTimeID int64) string {
	return fmt.Sprintf("%s:viewer_inflight:%s", prefix, rushScopeTag(showTimeID))
}

func userActiveKey(prefix string, showTimeID int64) string {
	return fmt.Sprintf("%s:user_active:%s", prefix, rushScopeTag(showTimeID))
}

func viewerActiveKey(prefix string, showTimeID int64) string {
	return fmt.Sprintf("%s:viewer_active:%s", prefix, rushScopeTag(showTimeID))
}

func quotaAvailableKey(prefix string, showTimeID int64) string {
	return fmt.Sprintf("%s:quota:%s", prefix, rushScopeTag(showTimeID))
}

func rushScopeTag(showTimeID int64) string {
	return fmt.Sprintf("{st:%d}", showTimeID)
}
