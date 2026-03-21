package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
)

func TestLoadGatewayConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configFile := filepath.Join(dir, "gateway-api.yaml")
	content := []byte(`
Name: gateway-api
Host: 0.0.0.0
Port: 8080
Telemetry:
  Name: gateway-api
  Endpoint: http://localhost:4318
Upstreams:
  - Name: user-api
    Http:
      Target: 127.0.0.1:8888
    Mappings:
      - Method: post
        Path: /user/login
  - Name: program-api
    Http:
      Target: 127.0.0.1:8889
    Mappings:
      - Method: post
        Path: /program/page
  - Name: order-api
    Http:
      Target: 127.0.0.1:8890
    Mappings:
      - Method: post
        Path: /order/create
Auth:
  ChannelMap:
    "0001": secret-0001
`)
	if err := os.WriteFile(configFile, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", configFile, err)
	}

	var c Config
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Auth.ChannelCodeHeader != "X-Channel-Code" {
		t.Fatalf("expected default channel header X-Channel-Code, got %q", c.Auth.ChannelCodeHeader)
	}

	if len(c.Upstreams) != 3 {
		t.Fatalf("expected 3 upstreams, got %d", len(c.Upstreams))
	}
}
