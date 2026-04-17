package config_test

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"

	"livepass/services/pay-api/internal/config"
)

func TestLoadPayAPIRuntimeConfigIncludesPrometheus(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "pay-api.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Port != 8892 {
		t.Fatalf("expected pay-api runtime port 8892, got %d", c.Port)
	}
	if c.Prometheus.Host == "" || c.Prometheus.Port == 0 {
		t.Fatalf("expected prometheus config to load, got host=%q port=%d", c.Prometheus.Host, c.Prometheus.Port)
	}
	if c.Timeout != 5000 {
		t.Fatalf("expected pay-api runtime timeout 5000, got %d", c.Timeout)
	}
	if c.PayRpc.Timeout != 5000 {
		t.Fatalf("expected pay rpc client timeout 5000, got %d", c.PayRpc.Timeout)
	}
}

func TestLoadPayAPIPerfConfigIncludesPrometheus(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "pay-api.perf.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Port != 8892 {
		t.Fatalf("expected pay-api perf port 8892, got %d", c.Port)
	}
	if c.Prometheus.Host == "" || c.Prometheus.Port == 0 {
		t.Fatalf("expected prometheus config to load, got host=%q port=%d", c.Prometheus.Host, c.Prometheus.Port)
	}
	if c.Timeout != 9000 {
		t.Fatalf("expected pay-api perf timeout 9000, got %d", c.Timeout)
	}
	if c.PayRpc.Timeout != 5000 {
		t.Fatalf("expected pay rpc client timeout 5000, got %d", c.PayRpc.Timeout)
	}
}
