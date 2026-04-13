package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDelayTaskOutboxSchema(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderCloseProjectRoot(t), "sql/order/sharding/d_delay_task_outbox.sql"))
	if err != nil {
		t.Fatalf("ReadFile(d_delay_task_outbox.sql) error = %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "UNIQUE KEY `uk_task_type_task_key` (`task_type`,`task_key`)") {
		t.Fatalf("expected uk_task_type_task_key on task_type,task_key, content=%s", text)
	}
	if !strings.Contains(text, "KEY `idx_dispatch_scan` (`published_status`,`task_type`,`status`,`id`)") {
		t.Fatalf("expected idx_dispatch_scan on published_status,task_type,status,id, content=%s", text)
	}
}
