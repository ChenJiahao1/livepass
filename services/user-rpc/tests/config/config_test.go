package config_test

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"

	"damai-go/services/user-rpc/internal/config"
)

func TestLoadUserRPCConfig(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "user-rpc.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.MySQL.DataSource == "" {
		t.Fatal("expected mysql datasource to be loaded")
	}

	if c.UserAuth.ChannelMap["0001"] == "" {
		t.Fatal("expected user auth channel map to be loaded")
	}
}
