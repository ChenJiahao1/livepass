package config

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestLoadProgramRPCConfigUsesDedicatedListenPort(t *testing.T) {
	t.Parallel()

	var c Config
	configFile := filepath.Join("..", "..", "etc", "program-rpc.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.ListenOn != "0.0.0.0:8083" {
		t.Fatalf("expected dedicated program-rpc listen address 0.0.0.0:8083, got %q", c.ListenOn)
	}
}
