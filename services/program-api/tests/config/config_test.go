package config_test

import (
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"

	"livepass/services/program-api/internal/config"
)

func TestLoadProgramAPIRuntimeConfig(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "program-api.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Port != 8889 {
		t.Fatalf("expected program-api runtime port 8889, got %d", c.Port)
	}
	if c.ProgramRpc.Etcd.Key != "program.rpc" {
		t.Fatalf("expected program rpc client key program.rpc, got %q", c.ProgramRpc.Etcd.Key)
	}
}

func TestLoadProgramAPIPerfConfig(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "program-api.perf.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Port != 8889 {
		t.Fatalf("expected program-api perf port 8889, got %d", c.Port)
	}
	if c.ProgramRpc.Etcd.Key != "program.rpc" {
		t.Fatalf("expected program rpc client key program.rpc, got %q", c.ProgramRpc.Etcd.Key)
	}
}
