package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDelayTaskOutboxSchemaHasDispatchIndex(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderCloseProjectRoot(t), "sql/order/sharding/d_delay_task_outbox.sql"))
	if err != nil {
		t.Fatalf("ReadFile(d_delay_task_outbox.sql) error = %v", err)
	}
	text := string(content)

	for _, column := range []string{
		"`task_status`",
		"`publish_attempts`",
		"`consume_attempts`",
		"`last_publish_error`",
		"`last_consume_error`",
		"`published_time`",
		"`processed_time`",
	} {
		if !strings.Contains(text, column) {
			t.Fatalf("expected column %s in schema, content=%s", column, text)
		}
	}
	if !strings.Contains(text, "UNIQUE KEY `uk_task_type_task_key` (`task_type`,`task_key`)") {
		t.Fatalf("expected uk_task_type_task_key on task_type,task_key, content=%s", text)
	}
	if !strings.Contains(text, "KEY `idx_dispatch_scan` (`task_status`,`task_type`,`status`,`id`)") {
		t.Fatalf("expected idx_dispatch_scan on task_status,task_type,status,id, content=%s", text)
	}
}
