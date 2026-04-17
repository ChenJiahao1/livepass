package logic

import (
	"testing"
	"time"

	"livepass/services/order-rpc/sharding"
)

func TestOrderNumberGeneratorAllocatesUniqueSequenceWithinSameSecond(t *testing.T) {
	gen := newOrderNumberGenerator(func() int64 { return 17 })
	now := time.Date(2026, time.March, 25, 10, 30, 15, 123000000, time.UTC)

	first := gen.Next(3001, now)
	second := gen.Next(3001, now.Add(2*time.Millisecond))

	if first == second {
		t.Fatalf("expected unique order numbers within same second, got duplicated %d", first)
	}

	firstParts, err := sharding.ParseOrderNumber(first)
	if err != nil {
		t.Fatalf("ParseOrderNumber(first) error = %v", err)
	}
	secondParts, err := sharding.ParseOrderNumber(second)
	if err != nil {
		t.Fatalf("ParseOrderNumber(second) error = %v", err)
	}

	if firstParts.WorkerID != 17 || secondParts.WorkerID != 17 {
		t.Fatalf("worker ids = (%d,%d), want both 17", firstParts.WorkerID, secondParts.WorkerID)
	}
	if firstParts.Sequence != 0 {
		t.Fatalf("first sequence = %d, want 0", firstParts.Sequence)
	}
	if secondParts.Sequence != 1 {
		t.Fatalf("second sequence = %d, want 1", secondParts.Sequence)
	}
	if firstParts.TimePart != secondParts.TimePart {
		t.Fatalf("time part mismatch, first=%d second=%d", firstParts.TimePart, secondParts.TimePart)
	}
}

func TestOrderNumberGeneratorResetsSequenceOnNextSecond(t *testing.T) {
	gen := newOrderNumberGenerator(func() int64 { return 23 })
	now := time.Date(2026, time.March, 25, 10, 30, 15, 999000000, time.UTC)

	first := gen.Next(3001, now)
	second := gen.Next(3001, now.Add(time.Second))

	firstParts, err := sharding.ParseOrderNumber(first)
	if err != nil {
		t.Fatalf("ParseOrderNumber(first) error = %v", err)
	}
	secondParts, err := sharding.ParseOrderNumber(second)
	if err != nil {
		t.Fatalf("ParseOrderNumber(second) error = %v", err)
	}

	if secondParts.Sequence != 0 {
		t.Fatalf("second sequence = %d, want 0 after second rollover", secondParts.Sequence)
	}
	if secondParts.TimePart <= firstParts.TimePart {
		t.Fatalf("expected later time part, first=%d second=%d", firstParts.TimePart, secondParts.TimePart)
	}
}

func TestOrderNumberGeneratorWaitsForClockAfterSequenceExhausted(t *testing.T) {
	start := time.Date(2026, time.March, 25, 10, 30, 15, 500000000, time.UTC)
	current := start

	gen := newOrderNumberGenerator(func() int64 { return 31 })
	gen.now = func() time.Time { return current }
	gen.sleep = func(d time.Duration) {
		current = current.Add(d)
	}
	gen.lastUnixSecond = start.UTC().Unix()
	gen.sequence = maxOrderNumberSequence

	startOrderNumber := sharding.BuildOrderNumber(3001, start, 31, 0)
	startParts, err := sharding.ParseOrderNumber(startOrderNumber)
	if err != nil {
		t.Fatalf("ParseOrderNumber(startOrderNumber) error = %v", err)
	}

	orderNumber := gen.Next(3001, start)
	parts, err := sharding.ParseOrderNumber(orderNumber)
	if err != nil {
		t.Fatalf("ParseOrderNumber(orderNumber) error = %v", err)
	}

	if got := current.UTC().Unix(); got != start.UTC().Unix()+1 {
		t.Fatalf("clock second = %d, want %d", got, start.UTC().Unix()+1)
	}
	if parts.Sequence != 0 {
		t.Fatalf("sequence = %d, want 0 after rollover", parts.Sequence)
	}
	if parts.TimePart != startParts.TimePart+1 {
		t.Fatalf("time part = %d, want %d", parts.TimePart, startParts.TimePart+1)
	}
}
