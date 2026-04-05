package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultOrderRushReconcileConfigUsesFastCompensationCadence(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderRushReconcileProjectRoot(t), "jobs/order-rush-reconcile/etc/order-rush-reconcile.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(order-rush-reconcile.yaml) error = %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "Interval: 1s") {
		t.Fatalf("expected Interval: 1s, content=%s", text)
	}
	if !strings.Contains(text, "BatchSize: 100") {
		t.Fatalf("expected BatchSize: 100, content=%s", text)
	}
}

func orderRushReconcileProjectRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller() failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
