package taskdef

import (
	"testing"
	"time"
)

func TestTaskKey(t *testing.T) {
	if got := TaskKey(91001); got != "order.close_timeout:91001" {
		t.Fatalf("TaskKey() = %s, want order.close_timeout:91001", got)
	}
}

func TestMarshalAndParse(t *testing.T) {
	body, err := Marshal(91001)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	payload, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if payload.OrderNumber != 91001 {
		t.Fatalf("payload.OrderNumber = %d, want 91001", payload.OrderNumber)
	}
}

func TestNewMessage(t *testing.T) {
	executeAt := time.Date(2026, time.April, 13, 15, 30, 0, 0, time.Local)

	message, err := NewMessage(91001, executeAt)
	if err != nil {
		t.Fatalf("NewMessage returned error: %v", err)
	}
	if message.Type != TaskTypeCloseTimeout {
		t.Fatalf("message.Type = %s, want %s", message.Type, TaskTypeCloseTimeout)
	}
	if message.Key != "order.close_timeout:91001" {
		t.Fatalf("message.Key = %s, want order.close_timeout:91001", message.Key)
	}
	if !message.ExecuteAt.Equal(executeAt) {
		t.Fatalf("message.ExecuteAt = %v, want %v", message.ExecuteAt, executeAt)
	}
	if string(message.Payload) != `{"orderNumber":91001}` {
		t.Fatalf("message.Payload = %s", string(message.Payload))
	}
}
