package preheatqueue

import (
	"testing"
	"time"
)

func TestMarshalRushInventoryPreheatPayloadEncodesExpectedOpenTimeAndLeadTime(t *testing.T) {
	expectedOpenTime := time.Date(2026, 12, 31, 18, 0, 0, 0, time.Local)

	body, err := MarshalRushInventoryPreheatPayload(30001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("MarshalRushInventoryPreheatPayload() error = %v", err)
	}

	payload, err := ParseRushInventoryPreheatPayload(body)
	if err != nil {
		t.Fatalf("ParseRushInventoryPreheatPayload() error = %v", err)
	}

	if payload.ShowTimeId != 30001 {
		t.Fatalf("expected showTimeId 30001, got %d", payload.ShowTimeId)
	}
	if payload.ExpectedRushSaleOpenTime != "2026-12-31 18:00:00" {
		t.Fatalf("expected expectedRushSaleOpenTime 2026-12-31 18:00:00, got %q", payload.ExpectedRushSaleOpenTime)
	}
	if payload.LeadTime != "5m0s" {
		t.Fatalf("expected leadTime 5m0s, got %q", payload.LeadTime)
	}
}

func TestRushInventoryPreheatTaskIDUsesShowTimeAndExpectedOpenTime(t *testing.T) {
	expectedOpenTime := time.Date(2026, 12, 31, 18, 0, 0, 0, time.Local)

	taskID := RushInventoryPreheatTaskID(30001, expectedOpenTime)

	if taskID != "rush-inventory-preheat:30001:20261231180000" {
		t.Fatalf("unexpected task id %q", taskID)
	}
}
