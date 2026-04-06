package closequeue

import (
	"testing"
	"time"
)

func TestCloseTimeoutTaskID(t *testing.T) {
	if got := CloseTimeoutTaskID(91001); got != "order-close:91001" {
		t.Fatalf("expected task id order-close:91001, got %s", got)
	}
}

func TestMarshalAndParseCloseTimeoutPayload(t *testing.T) {
	expireAt := time.Date(2026, time.March, 29, 19, 45, 0, 0, time.UTC)

	body, err := MarshalCloseTimeoutPayload(91001, expireAt)
	if err != nil {
		t.Fatalf("MarshalCloseTimeoutPayload returned error: %v", err)
	}

	payload, err := ParseCloseTimeoutPayload(body)
	if err != nil {
		t.Fatalf("ParseCloseTimeoutPayload returned error: %v", err)
	}
	if payload.OrderNumber != 91001 {
		t.Fatalf("expected order number 91001, got %d", payload.OrderNumber)
	}
	if payload.ExpireAt != "2026-03-29 19:45:00" {
		t.Fatalf("expected expireAt 2026-03-29 19:45:00, got %s", payload.ExpireAt)
	}
}

func TestVerifyAttemptTaskID(t *testing.T) {
	if got := VerifyAttemptTaskID(91001); got != "order-verify-attempt:91001" {
		t.Fatalf("expected task id order-verify-attempt:91001, got %s", got)
	}
}

func TestMarshalAndParseVerifyAttemptPayload(t *testing.T) {
	dueAt := time.Date(2026, time.April, 5, 20, 15, 0, 0, time.UTC)

	body, err := MarshalVerifyAttemptPayload(91001, 10001, dueAt)
	if err != nil {
		t.Fatalf("MarshalVerifyAttemptPayload returned error: %v", err)
	}

	payload, err := ParseVerifyAttemptPayload(body)
	if err != nil {
		t.Fatalf("ParseVerifyAttemptPayload returned error: %v", err)
	}
	if payload.OrderNumber != 91001 {
		t.Fatalf("expected order number 91001, got %d", payload.OrderNumber)
	}
	if payload.ProgramID != 10001 {
		t.Fatalf("expected program id 10001, got %d", payload.ProgramID)
	}
	if payload.DueAt != "2026-04-05 20:15:00" {
		t.Fatalf("expected dueAt 2026-04-05 20:15:00, got %s", payload.DueAt)
	}
}
