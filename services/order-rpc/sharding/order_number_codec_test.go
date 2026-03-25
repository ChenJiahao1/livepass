package sharding

import (
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
)

func TestLogicSlotByUserIDMatchesOrderNumber(t *testing.T) {
	userID := int64(20260324001)
	now := time.Date(2026, time.March, 24, 10, 30, 0, 0, time.UTC)

	orderNumber := BuildOrderNumber(userID, now, 17, 23)
	slotByUser := LogicSlotByUserID(userID)

	parts, err := ParseOrderNumber(orderNumber)
	if err != nil {
		t.Fatalf("ParseOrderNumber() error = %v", err)
	}
	if parts.Legacy {
		t.Fatalf("ParseOrderNumber() legacy = true, want false")
	}

	slotByOrder := int(parts.DBGene)<<5 | int(parts.TableGene)
	if slotByOrder != slotByUser {
		t.Fatalf("logic slot mismatch, order=%d user=%d", slotByOrder, slotByUser)
	}
	if parts.WorkerID != 17 {
		t.Fatalf("worker id = %d, want 17", parts.WorkerID)
	}
	if parts.Sequence != 23 {
		t.Fatalf("sequence = %d, want 23", parts.Sequence)
	}
}

func TestParseLegacyOrderNumberRequiresDirectoryLookup(t *testing.T) {
	node, err := snowflake.NewNode(7)
	if err != nil {
		t.Fatalf("snowflake.NewNode() error = %v", err)
	}

	parts, err := ParseOrderNumber(node.Generate().Int64())
	if err != nil {
		t.Fatalf("ParseOrderNumber() error = %v", err)
	}
	if !parts.Legacy {
		t.Fatalf("ParseOrderNumber() legacy = false, want true")
	}
}
