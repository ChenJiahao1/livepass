package rush

import (
	"testing"
	"time"
)

func TestMapAttemptRecordToPollMapsAcceptedAndProcessingToProcessing(t *testing.T) {
	now := time.Date(2026, 4, 5, 18, 0, 0, 0, time.Local)

	tests := []struct {
		name       string
		record     *AttemptRecord
		now        time.Time
		wantStatus int64
		wantDone   bool
	}{
		{
			name: "accepted is processing",
			record: &AttemptRecord{
				OrderNumber: 91001,
				State:       AttemptStateAccepted,
			},
			now:        now,
			wantStatus: PollOrderStatusProcessing,
			wantDone:   false,
		},
		{
			name: "processing is processing",
			record: &AttemptRecord{
				OrderNumber: 91002,
				State:       AttemptStateProcessing,
			},
			now:        now,
			wantStatus: PollOrderStatusProcessing,
			wantDone:   false,
		},
		{
			name: "success is success",
			record: &AttemptRecord{
				OrderNumber: 91003,
				State:       AttemptStateSuccess,
				ReasonCode:  AttemptReasonOrderCommitted,
			},
			now:        now,
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
			now:        now,
			wantStatus: PollOrderStatusFailed,
			wantDone:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotDone, err := MapAttemptRecordToPoll(tt.record, tt.now)
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
