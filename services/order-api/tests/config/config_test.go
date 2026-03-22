package config_test

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"

	"damai-go/services/order-api/internal/config"
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
}
