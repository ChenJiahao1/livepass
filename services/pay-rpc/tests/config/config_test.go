package config_test

import (
	"path/filepath"
	"testing"

	"livepass/services/pay-rpc/internal/config"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestLoadPayRPCConfigIncludesStaticXid(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "pay.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Xid.Provider != "static" {
		t.Fatalf("expected xid provider static, got %q", c.Xid.Provider)
	}
	if c.Xid.NodeId != 512 {
		t.Fatalf("expected xid node id 512, got %d", c.Xid.NodeId)
	}
	if len(c.Etcd.Hosts) == 0 || c.Etcd.Key == "" {
		t.Fatal("expected rpc etcd config to remain for service discovery")
	}
}

func TestLoadPayRPCPerfConfigIncludesStaticXid(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "pay.perf.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Xid.Provider != "static" {
		t.Fatalf("expected xid provider static, got %q", c.Xid.Provider)
	}
	if c.Xid.NodeId != 512 {
		t.Fatalf("expected xid node id 512, got %d", c.Xid.NodeId)
	}
	if len(c.Etcd.Hosts) == 0 || c.Etcd.Key == "" {
		t.Fatal("expected rpc etcd config to remain for service discovery")
	}
}
