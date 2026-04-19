package rush

import (
	"testing"
	"time"
)

func TestMapAttemptRecordToPollMapsAcceptedAndProcessingToProcessing(t *testing.T) {
	now := time.Date(2026, 4, 5, 18, 0, 0, 0, time.Local)

	for _, state := range []string{AttemptStatePending, AttemptStateProcessing} {
		t.Run(state, func(t *testing.T) {
			status, done, err := MapAttemptRecordToPoll(&AttemptRecord{
				OrderNumber: 91001,
				UserID:      92001,
				State:       state,
			}, now)
			if err != nil {
				t.Fatalf("MapAttemptRecordToPoll(%s) error = %v", state, err)
			}
			if status != PollOrderStatusProcessing || done {
				t.Fatalf("expected queueing for %s, got status=%d done=%t", state, status, done)
			}
		})
	}
}

func TestMapAttemptRecordToPollMapsSuccessAndFailedToTerminalStates(t *testing.T) {
	now := time.Date(2026, 4, 5, 18, 0, 0, 0, time.Local)

	tests := []struct {
		name       string
		record     *AttemptRecord
		wantStatus int64
		wantDone   bool
	}{
		{
			name: "success is success",
			record: &AttemptRecord{
				OrderNumber: 91003,
				State:       AttemptStateSuccess,
				ReasonCode:  AttemptReasonOrderCommitted,
			},
			wantStatus: PollOrderStatusSuccess,
			wantDone:   true,
		},
		{
			name: "failed is failed",
			record: &AttemptRecord{
				OrderNumber: 91004,
				State:       AttemptStateFailed,
				ReasonCode:  AttemptReasonQuotaExhausted,
			},
			wantStatus: PollOrderStatusFailed,
			wantDone:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotDone, err := MapAttemptRecordToPoll(tt.record, now)
			if err != nil {
				t.Fatalf("MapAttemptRecordToPoll() error = %v", err)
			}
			if gotStatus != tt.wantStatus || gotDone != tt.wantDone {
				t.Fatalf(
					"MapAttemptRecordToPoll() = (%d, %t), want (%d, %t)",
					gotStatus,
					gotDone,
					tt.wantStatus,
					tt.wantDone,
				)
			}
		})
	}
}

func TestMapAttemptRecordToPollRejectsLegacyVerifyingState(t *testing.T) {
	_, _, err := MapAttemptRecordToPoll(&AttemptRecord{
		OrderNumber: 91005,
		State:       "VERIFYING",
	}, time.Date(2026, 4, 5, 18, 0, 0, 0, time.Local))
	if err == nil {
		t.Fatalf("expected error for legacy VERIFYING state")
	}
}
