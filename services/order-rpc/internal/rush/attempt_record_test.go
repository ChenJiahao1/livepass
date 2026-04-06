package rush

import (
	"testing"
	"time"
)

func TestMapAttemptRecordToPoll(t *testing.T) {
	now := time.Date(2026, 4, 5, 18, 0, 0, 0, time.Local)

	tests := []struct {
		name       string
		record     *AttemptRecord
		now        time.Time
		wantStatus int64
		wantDone   bool
	}{
		{
			name: "pending publish before deadline is processing",
			record: &AttemptRecord{
				OrderNumber:    91001,
				State:          AttemptStatePendingPublish,
				UserDeadlineAt: now.Add(3 * time.Second),
			},
			now:        now,
			wantStatus: PollOrderStatusProcessing,
			wantDone:   false,
		},
		{
			name: "processing after deadline is verifying",
			record: &AttemptRecord{
				OrderNumber:    91002,
				State:          AttemptStateProcessing,
				UserDeadlineAt: now.Add(-time.Second),
			},
			now:        now,
			wantStatus: PollOrderStatusVerifying,
			wantDone:   false,
		},
		{
			name: "committed is success",
			record: &AttemptRecord{
				OrderNumber: 91003,
				State:       AttemptStateCommitted,
				ReasonCode:  AttemptReasonOrderCommitted,
			},
			now:        now,
			wantStatus: PollOrderStatusSuccess,
			wantDone:   true,
		},
		{
			name: "released is failed",
			record: &AttemptRecord{
				OrderNumber: 91004,
				State:       AttemptStateReleased,
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
