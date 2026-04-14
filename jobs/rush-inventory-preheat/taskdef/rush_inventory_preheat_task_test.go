package taskdef

import (
	"testing"
	"time"
)

func TestMarshalEncodesExpectedOpenTimeAndLeadTime(t *testing.T) {
	expectedOpenTime := time.Date(2026, 12, 31, 18, 0, 0, 0, time.Local)

	body, err := Marshal(20001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	payload, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if payload.ProgramId != 20001 {
		t.Fatalf("expected programId 20001, got %d", payload.ProgramId)
	}
	if payload.ExpectedRushSaleOpenTime != "2026-12-31 18:00:00" {
		t.Fatalf("expected expectedRushSaleOpenTime 2026-12-31 18:00:00, got %q", payload.ExpectedRushSaleOpenTime)
	}
	if payload.LeadTime != "5m0s" {
		t.Fatalf("expected leadTime 5m0s, got %q", payload.LeadTime)
	}
}

func TestTaskKeyUsesProgramAndExpectedOpenTime(t *testing.T) {
	expectedOpenTime := time.Date(2026, 12, 31, 18, 0, 0, 0, time.Local)

	taskKey := TaskKey(20001, expectedOpenTime)

	if taskKey != "program.rush_inventory_preheat:20001:20261231180000" {
		t.Fatalf("unexpected task key %q", taskKey)
	}
}

func TestNewMessageUsesLeadTimeAsDispatchTime(t *testing.T) {
	expectedOpenTime := time.Date(2026, 12, 31, 18, 0, 0, 0, time.Local)

	message, err := NewMessage(20001, expectedOpenTime, 5*time.Minute)
	if err != nil {
		t.Fatalf("NewMessage() error = %v", err)
	}

	if message.Type != TaskTypeRushInventoryPreheat {
		t.Fatalf("expected type %q, got %q", TaskTypeRushInventoryPreheat, message.Type)
	}
	if message.Key != "program.rush_inventory_preheat:20001:20261231180000" {
		t.Fatalf("unexpected message key %q", message.Key)
	}
	if message.ExecuteAt.Format(time.DateTime) != "2026-12-31 17:55:00" {
		t.Fatalf("expected executeAt 2026-12-31 17:55:00, got %s", message.ExecuteAt.Format(time.DateTime))
	}
}
