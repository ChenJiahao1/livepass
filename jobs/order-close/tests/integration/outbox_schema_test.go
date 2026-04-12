package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutboxSchemaIncludesDispatchScanIndex(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderCloseProjectRoot(t), "sql/order/sharding/d_order_outbox.sql"))
	if err != nil {
		t.Fatalf("ReadFile(d_order_outbox.sql) error = %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "KEY `idx_dispatch_scan` (`published_status`,`event_type`,`status`,`id`)") {
		t.Fatalf("expected idx_dispatch_scan on published_status,event_type,status,id, content=%s", text)
	}
}
