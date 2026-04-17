package config_test

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"

	"livepass/services/user-rpc/internal/config"
)

func TestLoadUserRPCConfig(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "user.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.MySQL.DataSource == "" {
		t.Fatal("expected mysql datasource to be loaded")
	}
	if c.MySQL.MaxOpenConns != 10 {
		t.Fatalf("expected mysql max open conns 10, got %d", c.MySQL.MaxOpenConns)
	}
	if c.MySQL.MaxIdleConns != 4 {
		t.Fatalf("expected mysql max idle conns 4, got %d", c.MySQL.MaxIdleConns)
	}

	if c.UserAuth.AccessSecret == "" {
		t.Fatal("expected user auth access secret to be loaded")
	}
}

func TestLoadUserRPCConfigIncludesStaticXid(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "user.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Xid.Provider != "static" {
		t.Fatalf("expected xid provider static, got %q", c.Xid.Provider)
	}
	if c.Xid.NodeId != 0 {
		t.Fatalf("expected xid node id 0, got %d", c.Xid.NodeId)
	}
	if len(c.Etcd.Hosts) == 0 || c.Etcd.Key == "" {
		t.Fatal("expected rpc etcd config to remain for service discovery")
	}
}
