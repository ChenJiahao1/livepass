package outbox

import (
	"testing"

	"livepass/pkg/delaytask"
)

func TestDispatchableStatusesExcludeProcessed(t *testing.T) {
	statuses := DispatchableStatuses()
	for _, status := range statuses {
		if status == delaytask.OutboxTaskStatusProcessed {
			t.Fatalf("DispatchableStatuses() includes processed")
		}
	}
	if len(statuses) != 3 {
		t.Fatalf("DispatchableStatuses() len = %d, want 3", len(statuses))
	}
}
