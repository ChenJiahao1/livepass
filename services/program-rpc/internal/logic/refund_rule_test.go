package logic

import (
	"testing"
	"time"
)

func TestEvaluateRefundStages(t *testing.T) {
	rule := `{"version":1,"stages":[{"beforeMinutes":1440,"refundPercent":80}]}`

	got, err := evaluateRefundRule(rule, mustParseRefundTestTime("2026-12-31 19:30:00"), mustParseRefundTestTime("2026-12-30 18:30:00"), 598)
	if err != nil {
		t.Fatalf("evaluateRefundRule returned error: %v", err)
	}
	if !got.AllowRefund || got.RefundPercent != 80 || got.RefundAmount != 478 {
		t.Fatalf("unexpected refund result: %+v", got)
	}
}

func mustParseRefundTestTime(value string) time.Time {
	parsed, err := time.ParseInLocation(programDateTimeLayout, value, time.Local)
	if err != nil {
		panic(err)
	}

	return parsed
}
