package config_test

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"

	"livepass/services/user-api/internal/config"
)

func TestLoadUserAPIRuntimeConfig(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "user-api.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Port != 8888 {
		t.Fatalf("expected user-api runtime port 8888, got %d", c.Port)
	}
	if c.UserRpc.Etcd.Key != "user.rpc" {
		t.Fatalf("expected user rpc client key user.rpc, got %q", c.UserRpc.Etcd.Key)
	}
	if c.GatewayAuth.Secret != "local-gateway-internal-secret" {
		t.Fatalf("expected gateway auth secret to load, got %q", c.GatewayAuth.Secret)
	}
}

func TestLoadUserAPIPerfConfig(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "user-api.perf.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Port != 8888 {
		t.Fatalf("expected user-api perf port 8888, got %d", c.Port)
	}
	if c.UserRpc.Etcd.Key != "user.rpc" {
		t.Fatalf("expected user rpc client key user.rpc, got %q", c.UserRpc.Etcd.Key)
	}
	if c.GatewayAuth.Secret != "local-gateway-internal-secret" {
		t.Fatalf("expected gateway auth secret to load, got %q", c.GatewayAuth.Secret)
	}
}
