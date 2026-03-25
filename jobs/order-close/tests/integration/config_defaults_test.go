package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultOrderCloseConfigCoversAllLogicSlots(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderCloseProjectRoot(t), "jobs/order-close/etc/order-close.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(order-close.yaml) error = %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "ScanSlotStart: 0") {
		t.Fatalf("expected ScanSlotStart: 0, content=%s", text)
	}
	if !strings.Contains(text, "ScanSlotEnd: 1023") {
		t.Fatalf("expected ScanSlotEnd: 1023, content=%s", text)
	}
	if !strings.Contains(text, "CheckpointSlot: 0") {
		t.Fatalf("expected CheckpointSlot: 0, content=%s", text)
	}
}

func orderCloseProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller() failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
