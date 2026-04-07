package rush

import "fmt"

const (
	defaultAttemptPrefix = "damai-go:order:rush"

	attemptFieldOrderNumber       = "order_number"
	attemptFieldUserID            = "user_id"
	attemptFieldProgramID         = "program_id"
	attemptFieldShowTimeID        = "show_time_id"
	attemptFieldTicketCategoryID  = "ticket_category_id"
	attemptFieldViewerIDs         = "viewer_ids"
	attemptFieldTicketCount       = "ticket_count"
	attemptFieldGeneration        = "generation"
	attemptFieldSaleWindowEndAt   = "sale_window_end_at"
	attemptFieldShowEndAt         = "show_end_at"
	attemptFieldTokenFingerprint  = "token_fingerprint"
	attemptFieldState             = "state"
	attemptFieldReasonCode        = "reason_code"
	attemptFieldCommitCutoffAt    = "commit_cutoff_at"
	attemptFieldUserDeadlineAt    = "user_deadline_at"
	attemptFieldProcessingEpoch   = "processing_epoch"
	attemptFieldProcessingStartAt = "processing_started_at"
	attemptFieldVerifyStartedAt   = "verify_started_at"
	attemptFieldLastDBProbeAt     = "last_db_probe_at"
	attemptFieldDBProbeAttempts   = "db_probe_attempts"
	attemptFieldCreatedAt         = "created_at"
	attemptFieldTransitionAt      = "last_transition_at"
)

func attemptRecordKey(prefix string, showTimeID int64, generation string, orderNumber int64) string {
	return fmt.Sprintf("%s:%s:attempt:%d", prefix, rushScopeTag(showTimeID, generation), orderNumber)
}

func userInflightKey(prefix string, showTimeID int64, generation string, userID int64) string {
	return fmt.Sprintf("%s:%s:user_inflight:%d", prefix, rushScopeTag(showTimeID, generation), userID)
}

func viewerInflightKey(prefix string, showTimeID int64, generation string, viewerID int64) string {
	return fmt.Sprintf("%s:%s:viewer_inflight:%d", prefix, rushScopeTag(showTimeID, generation), viewerID)
}

func userActiveKey(prefix string, showTimeID int64, generation string, userID int64) string {
	return fmt.Sprintf("%s:%s:user_active:%d", prefix, rushScopeTag(showTimeID, generation), userID)
}

func viewerActiveKey(prefix string, showTimeID int64, generation string, viewerID int64) string {
	return fmt.Sprintf("%s:%s:viewer_active:%d", prefix, rushScopeTag(showTimeID, generation), viewerID)
}

func quotaAvailableKey(prefix string, showTimeID int64, generation string, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:%s:quota:%d", prefix, rushScopeTag(showTimeID, generation), ticketCategoryID)
}

func seatOccupiedKey(prefix string, showTimeID int64, generation string, orderNumber int64) string {
	return fmt.Sprintf("%s:%s:seat_occupied:%d", prefix, rushScopeTag(showTimeID, generation), orderNumber)
}

func userFingerprintIndexKey(prefix string, showTimeID int64, generation string, userID int64) string {
	return fmt.Sprintf("%s:%s:fingerprint:%d", prefix, rushScopeTag(showTimeID, generation), userID)
}

func rushScopeTag(showTimeID int64, generation string) string {
	return fmt.Sprintf("{st:%d:g:%s}", showTimeID, normalizeRushGeneration(showTimeID, generation))
}

func normalizeRushGeneration(showTimeID int64, generation string) string {
	if generation == "" {
		return BuildRushGeneration(showTimeID)
	}
	return generation
}
