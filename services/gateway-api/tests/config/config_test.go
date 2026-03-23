package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/gateway"

	"damai-go/services/gateway-api/internal/config"
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

	var c config.Config
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

func TestLoadGatewayRuntimeConfigIncludesPrometheus(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "gateway-api.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Prometheus.Host == "" || c.Prometheus.Port == 0 {
		t.Fatalf("expected prometheus config to load, got host=%q port=%d", c.Prometheus.Host, c.Prometheus.Port)
	}

	orderAPIUpstream := findGatewayUpstream(t, c.Upstreams, "order-api")
	if orderAPIUpstream.Http == nil {
		t.Fatalf("expected order-api http upstream to be configured")
	}
	if orderAPIUpstream.Http.Timeout != 6000 {
		t.Fatalf("expected order-api timeout 6000, got %d", orderAPIUpstream.Http.Timeout)
	}
}

func TestLoadGatewayPerfConfigExtendsOrderTimeout(t *testing.T) {
	t.Parallel()

	var c config.Config
	configFile := filepath.Join("..", "..", "etc", "gateway-api.perf.yaml")
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Prometheus.Host == "" || c.Prometheus.Port == 0 {
		t.Fatalf("expected prometheus config to load, got host=%q port=%d", c.Prometheus.Host, c.Prometheus.Port)
	}

	orderAPIUpstream := findGatewayUpstream(t, c.Upstreams, "order-api")
	if orderAPIUpstream.Http == nil {
		t.Fatalf("expected order-api http upstream to be configured")
	}
	if orderAPIUpstream.Http.Timeout != 10000 {
		t.Fatalf("expected order-api timeout 10000, got %d", orderAPIUpstream.Http.Timeout)
	}
}

func findGatewayUpstream(t *testing.T, upstreams []gateway.Upstream, name string) gateway.Upstream {
	t.Helper()

	for _, upstream := range upstreams {
		if upstream.Name == name {
			return upstream
		}
	}

	t.Fatalf("expected upstream %q to exist", name)
	return gateway.Upstream{}
}
