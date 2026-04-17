package config_test

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"

	"livepass/services/order-api/internal/config"
)

func TestLoadOrderAPIRuntimeConfigIncludesPrometheus(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "order-api.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Prometheus.Host == "" || c.Prometheus.Port == 0 {
		t.Fatalf("expected prometheus config to load, got host=%q port=%d", c.Prometheus.Host, c.Prometheus.Port)
	}
	if c.Timeout != 5000 {
		t.Fatalf("expected order-api runtime timeout 5000, got %d", c.Timeout)
	}
	if c.OrderRpc.Timeout != 5000 {
		t.Fatalf("expected order rpc client timeout 5000, got %d", c.OrderRpc.Timeout)
	}
}

func TestLoadOrderAPIPerfConfigIncludesPrometheus(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "order-api.perf.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Prometheus.Host == "" || c.Prometheus.Port == 0 {
		t.Fatalf("expected prometheus config to load, got host=%q port=%d", c.Prometheus.Host, c.Prometheus.Port)
	}
	if c.Timeout != 9000 {
		t.Fatalf("expected order-api perf timeout 9000, got %d", c.Timeout)
	}
	if c.OrderRpc.Timeout != 5000 {
		t.Fatalf("expected order rpc client timeout 5000, got %d", c.OrderRpc.Timeout)
	}
}
