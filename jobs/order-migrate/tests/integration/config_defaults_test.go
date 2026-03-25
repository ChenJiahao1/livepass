package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	jobconfig "damai-go/jobs/order-migrate/internal/config"

	"gopkg.in/yaml.v3"
)

func TestDefaultOrderMigrateConfigDeclaresAllShardKeysUsedByRouteMap(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(orderMigrateProjectRoot(t), "jobs/order-migrate/etc/order-migrate.yaml"))
	if err != nil {
		t.Fatalf("ReadFile(order-migrate.yaml) error = %v", err)
	}

	var cfg jobconfig.Config
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal(order-migrate.yaml) error = %v", err)
	}

	if _, ok := cfg.Shards["order-db-0"]; !ok {
		t.Fatalf("expected default config to declare shard order-db-0")
	}
	if _, ok := cfg.Shards["order-db-1"]; !ok {
		t.Fatalf("expected default config to declare shard order-db-1")
	}
}
