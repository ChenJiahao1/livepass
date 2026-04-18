package event

import "testing"

func TestOrderCreateEventPartitionKeyUsesOrderNumber(t *testing.T) {
	event := &OrderCreateEvent{
		OrderNumber:      91001,
		ShowTimeID:       30001,
		TicketCategoryID: 40001,
	}

	if got := event.PartitionKey(); got != "91001" {
		t.Fatalf("PartitionKey() = %q, want order number", got)
	}
}
