package rush

import "fmt"

const (
	defaultAttemptPrefix = "damai-go:order:rush"

	attemptFieldOrderNumber       = "order_number"
	attemptFieldUserID            = "user_id"
	attemptFieldProgramID         = "program_id"
	attemptFieldTicketCategoryID  = "ticket_category_id"
	attemptFieldViewerIDs         = "viewer_ids"
	attemptFieldTicketCount       = "ticket_count"
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
	attemptFieldNextDBProbeAt     = "next_db_probe_at"
	attemptFieldCreatedAt         = "created_at"
	attemptFieldTransitionAt      = "last_transition_at"
)

func attemptRecordKey(prefix string, orderNumber int64) string {
	return fmt.Sprintf("%s:attempt:%d", prefix, orderNumber)
}

func userInflightKey(prefix string, programID, userID int64) string {
	return fmt.Sprintf("%s:inflight:program:%d:user:%d", prefix, programID, userID)
}

func viewerInflightKey(prefix string, programID, viewerID int64) string {
	return fmt.Sprintf("%s:inflight:program:%d:viewer:%d", prefix, programID, viewerID)
}

func quotaAvailableKey(prefix string, programID, ticketCategoryID int64) string {
	return fmt.Sprintf("%s:quota:%d:%d", prefix, programID, ticketCategoryID)
}

func orderProgressIndexKey(prefix string, programID int64) string {
	return fmt.Sprintf("%s:order-progress:%d", prefix, programID)
}

func userFingerprintIndexKey(prefix string, userID int64) string {
	return fmt.Sprintf("%s:fingerprint:user:%d", prefix, userID)
}
