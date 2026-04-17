package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"livepass/jobs/rush-inventory-preheat/internal/config"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestLoadRushInventoryPreheatDispatcherConfigUsesShardStorage(t *testing.T) {
	t.Parallel()

	var c config.DispatcherConfig
	configFile := filepath.Join("..", "..", "etc", "rush-inventory-preheat-dispatcher.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Interval != 5*time.Second {
		t.Fatalf("expected interval 5s, got %s", c.Interval)
	}
	if c.BatchSize != 200 {
		t.Fatalf("expected batch size 200, got %d", c.BatchSize)
	}
	if len(c.Shards) != 1 {
		t.Fatalf("expected exactly one shard config, got %d", len(c.Shards))
	}

	programShard, ok := c.Shards["program-db-0"]
	if !ok {
		t.Fatalf("expected shard program-db-0 to be configured")
	}
	if programShard.DataSource == "" {
		t.Fatalf("expected shard program-db-0 datasource to be configured")
	}
	if c.Asynq.Queue != "rush_inventory_preheat" {
		t.Fatalf("expected queue rush_inventory_preheat, got %q", c.Asynq.Queue)
	}
}

func TestLoadRushInventoryPreheatWorkerConfigUsesDedicatedDependencies(t *testing.T) {
	t.Parallel()

	var c config.WorkerConfig
	configFile := filepath.Join("..", "..", "etc", "rush-inventory-preheat-worker.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.MySQL.DataSource == "" {
		t.Fatalf("expected worker mysql datasource to be configured")
	}
	if c.OrderRpc.Etcd.Key != "order.rpc" {
		t.Fatalf("expected order rpc key order.rpc, got %q", c.OrderRpc.Etcd.Key)
	}
	if c.ProgramRpc.Etcd.Key != "program.rpc" {
		t.Fatalf("expected program rpc key program.rpc, got %q", c.ProgramRpc.Etcd.Key)
	}
}
