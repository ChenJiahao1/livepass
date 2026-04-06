package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultOrderCloseWorkerConfigUsesRushVerifyQueueSettings(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderCloseWorkerProjectRoot(t), "jobs/order-close-worker/etc/order-close-worker.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(order-close-worker.yaml) error = %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "Queue: order_close") {
		t.Fatalf("expected Queue: order_close, content=%s", text)
	}
	if !strings.Contains(text, "Concurrency: 16") {
		t.Fatalf("expected Concurrency: 16, content=%s", text)
	}
	if !strings.Contains(text, "ShutdownTimeout: 10s") {
		t.Fatalf("expected ShutdownTimeout: 10s, content=%s", text)
	}
}

func orderCloseWorkerProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller() failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
