package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultOrderCloseConfigCoversAllLogicSlots(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderCloseProjectRoot(t), "jobs/order-close/etc/order-close-dispatcher.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(order-close-dispatcher.yaml) error = %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "Shards:") {
		t.Fatalf("expected Shards config, content=%s", text)
	}
	if !strings.Contains(text, "order-db-0:") {
		t.Fatalf("expected order-db-0 shard config, content=%s", text)
	}
	if !strings.Contains(text, "order-db-1:") {
		t.Fatalf("expected order-db-1 shard config, content=%s", text)
	}
}

func TestDefaultOrderCloseConfigUsesCompensationCadence(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderCloseProjectRoot(t), "jobs/order-close/etc/order-close-dispatcher.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(order-close-dispatcher.yaml) error = %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "Interval: 1m") {
		t.Fatalf("expected Interval: 1m, content=%s", text)
	}
	if strings.Contains(text, "BatchSize:") {
		t.Fatalf("expected BatchSize to be removed, content=%s", text)
	}
	if !strings.Contains(text, "Queue: order_close") {
		t.Fatalf("expected Queue: order_close, content=%s", text)
	}
}

func TestDefaultOrderCloseWorkerConfigUsesWorkerQueueSettings(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderCloseProjectRoot(t), "jobs/order-close/etc/order-close-worker.yaml"))
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

func orderCloseProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller() failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
