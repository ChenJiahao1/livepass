package programcache

import (
	"testing"
	"time"
)

func TestInvalidationMessageRoundTrip(t *testing.T) {
	publishedAt := time.Date(2026, 4, 11, 9, 30, 0, 0, time.UTC)
	msg := InvalidationMessage{
		Version:     "v1",
		Service:     "program-rpc",
		InstanceID:  "instance-1",
		PublishedAt: publishedAt,
		Entries: []InvalidationEntry{
			{
				Cache:     cacheProgramDetail,
				ProgramID: 10001,
			},
			{
				Cache: cacheCategorySnapshot,
			},
		},
	}

	payload, err := MarshalInvalidationMessage(msg)
	if err != nil {
		t.Fatalf("MarshalInvalidationMessage returned error: %v", err)
	}

	decoded, err := ParseInvalidationMessage(payload)
	if err != nil {
		t.Fatalf("ParseInvalidationMessage returned error: %v", err)
	}

	if decoded.Version != msg.Version {
		t.Fatalf("expected version %q, got %q", msg.Version, decoded.Version)
	}
	if decoded.Service != msg.Service {
		t.Fatalf("expected service %q, got %q", msg.Service, decoded.Service)
	}
	if decoded.InstanceID != msg.InstanceID {
		t.Fatalf("expected instance id %q, got %q", msg.InstanceID, decoded.InstanceID)
	}
	if !decoded.PublishedAt.Equal(msg.PublishedAt) {
		t.Fatalf("expected published_at %v, got %v", msg.PublishedAt, decoded.PublishedAt)
	}
	if len(decoded.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(decoded.Entries))
	}
	if decoded.Entries[0].Cache != cacheProgramDetail || decoded.Entries[0].ProgramID != 10001 {
		t.Fatalf("unexpected first entry: %+v", decoded.Entries[0])
	}
	if decoded.Entries[1].Cache != cacheCategorySnapshot || decoded.Entries[1].ProgramID != 0 {
		t.Fatalf("unexpected second entry: %+v", decoded.Entries[1])
	}
}
