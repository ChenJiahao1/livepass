package config_test

import (
	"path/filepath"
	"testing"
	"time"

	"damai-go/services/order-rpc/internal/config"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestLoadOrderRPCRuntimeConfigIncludesTimeoutBudgetAndMySQLPool(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "order-rpc.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Timeout != 5500 {
		t.Fatalf("expected order-rpc runtime timeout 5500, got %d", c.Timeout)
	}
	if c.ProgramRpc.Timeout != 4000 {
		t.Fatalf("expected program rpc timeout 4000, got %d", c.ProgramRpc.Timeout)
	}
	if c.UserRpc.Timeout != 4000 {
		t.Fatalf("expected user rpc timeout 4000, got %d", c.UserRpc.Timeout)
	}
	if c.PayRpc.Timeout != 4000 {
		t.Fatalf("expected pay rpc timeout 4000, got %d", c.PayRpc.Timeout)
	}
	if c.MySQL.MaxOpenConns != 24 {
		t.Fatalf("expected mysql max open conns 24, got %d", c.MySQL.MaxOpenConns)
	}
	if c.MySQL.MaxIdleConns != 8 {
		t.Fatalf("expected mysql max idle conns 8, got %d", c.MySQL.MaxIdleConns)
	}
	if c.MySQL.ConnMaxLifetime != 3*time.Minute {
		t.Fatalf("expected mysql conn max lifetime 3m, got %s", c.MySQL.ConnMaxLifetime)
	}
	if c.MySQL.ConnMaxIdleTime != time.Minute {
		t.Fatalf("expected mysql conn max idle time 1m, got %s", c.MySQL.ConnMaxIdleTime)
	}
	if c.Kafka.TopicPartitions != 5 {
		t.Fatalf("expected kafka topic partitions 5, got %d", c.Kafka.TopicPartitions)
	}
	if c.Kafka.ConsumerWorkers != 1 {
		t.Fatalf("expected kafka consumer workers 1, got %d", c.Kafka.ConsumerWorkers)
	}
}

func TestLoadOrderRPCPerfConfigIncludesTimeoutBudgetAndMySQLPool(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "order-rpc.perf.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Timeout != 5500 {
		t.Fatalf("expected order-rpc perf timeout 5500, got %d", c.Timeout)
	}
	if c.ProgramRpc.Timeout != 4000 {
		t.Fatalf("expected program rpc timeout 4000, got %d", c.ProgramRpc.Timeout)
	}
	if c.UserRpc.Timeout != 4000 {
		t.Fatalf("expected user rpc timeout 4000, got %d", c.UserRpc.Timeout)
	}
	if c.PayRpc.Timeout != 4000 {
		t.Fatalf("expected pay rpc timeout 4000, got %d", c.PayRpc.Timeout)
	}
	if c.MySQL.MaxOpenConns != 24 {
		t.Fatalf("expected mysql max open conns 24, got %d", c.MySQL.MaxOpenConns)
	}
	if c.MySQL.MaxIdleConns != 8 {
		t.Fatalf("expected mysql max idle conns 8, got %d", c.MySQL.MaxIdleConns)
	}
	if c.MySQL.ConnMaxLifetime != 5*time.Minute {
		t.Fatalf("expected mysql conn max lifetime 5m, got %s", c.MySQL.ConnMaxLifetime)
	}
	if c.MySQL.ConnMaxIdleTime != 2*time.Minute {
		t.Fatalf("expected mysql conn max idle time 2m, got %s", c.MySQL.ConnMaxIdleTime)
	}
	if c.Kafka.TopicPartitions != 5 {
		t.Fatalf("expected kafka topic partitions 5, got %d", c.Kafka.TopicPartitions)
	}
	if c.Kafka.ConsumerWorkers != 1 {
		t.Fatalf("expected kafka consumer workers 1, got %d", c.Kafka.ConsumerWorkers)
	}
}
