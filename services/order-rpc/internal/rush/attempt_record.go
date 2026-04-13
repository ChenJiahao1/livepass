package rush

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"damai-go/pkg/xerr"
)

type AttemptRecord struct {
	OrderNumber      int64
	UserID           int64
	ProgramID        int64
	ShowTimeID       int64
	TicketCategoryID int64
	ViewerIDs        []int64
	TicketCount      int64
	SaleWindowEndAt  time.Time
	ShowEndAt        time.Time
	TokenFingerprint string

	State           string
	ReasonCode      string
	AcceptedAt      time.Time
	FinishedAt      time.Time
	PublishAttempts int64

	ProcessingEpoch     int64
	ProcessingStartedAt time.Time

	CreatedAt        time.Time
	LastTransitionAt time.Time
}

func MapAttemptRecordToPoll(record *AttemptRecord, now time.Time) (status int64, done bool, err error) {
	if record == nil {
		return 0, false, xerr.ErrInvalidParam
	}
	if now.IsZero() {
		now = time.Now()
	}

	switch record.State {
	case AttemptStateAccepted, AttemptStateProcessing:
		return PollOrderStatusProcessing, false, nil
	case AttemptStateSuccess:
		if record.ReasonCode != "" && record.ReasonCode != AttemptReasonOrderCommitted {
			return 0, false, fmt.Errorf("invalid committed reason code: %s", record.ReasonCode)
		}
		return PollOrderStatusSuccess, true, nil
	case AttemptStateFailed:
		return PollOrderStatusFailed, true, nil
	default:
		return 0, false, fmt.Errorf("unknown attempt state: %s", record.State)
	}
}

func parseInt64CSV(raw string) ([]int64, error) {
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	values := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}

	return values, nil
}

func formatInt64CSV(values []int64) string {
	if len(values) == 0 {
		return ""
	}

	parts := make([]string, 0, len(values))
	seen := make(map[int64]struct{}, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		parts = append(parts, strconv.FormatInt(v, 10))
	}

	return strings.Join(parts, ",")
}
