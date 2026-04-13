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
  - Name: pay-api
    Http:
      Target: 127.0.0.1:8892
    Mappings:
      - Method: post
        Path: /pay/detail
Auth:
  AccessSecret: secret-0001
InternalAuth:
  Secret: gateway-internal-secret
`)
	if err := os.WriteFile(configFile, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", configFile, err)
	}

	var c config.Config
	if err := conf.Load(configFile, &c); err != nil {
		t.Fatalf("load %s: %v", configFile, err)
	}

	if c.Auth.AccessSecret != "secret-0001" {
		t.Fatalf("expected access secret to load, got %q", c.Auth.AccessSecret)
	}
	if c.InternalAuth.Secret != "gateway-internal-secret" {
		t.Fatalf("expected internal auth secret to load, got %q", c.InternalAuth.Secret)
	}

	if len(c.Upstreams) != 4 {
		t.Fatalf("expected 4 upstreams, got %d", len(c.Upstreams))
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
	if c.Timeout <= orderAPIUpstream.Http.Timeout {
		t.Fatalf("expected gateway timeout > order-api timeout, got gateway=%d order-api=%d", c.Timeout, orderAPIUpstream.Http.Timeout)
	}
	if orderAPIUpstream.Http.Timeout != 6000 {
		t.Fatalf("expected order-api timeout 6000, got %d", orderAPIUpstream.Http.Timeout)
	}
	assertGatewayMappingExists(t, orderAPIUpstream, "/order/account/order/count")
	assertGatewayMappingExists(t, orderAPIUpstream, "/order/get/cache")

	payAPIUpstream := findGatewayUpstream(t, c.Upstreams, "pay-api")
	if payAPIUpstream.Http == nil {
		t.Fatalf("expected pay-api http upstream to be configured")
	}
	if payAPIUpstream.Http.Target != "127.0.0.1:8892" {
		t.Fatalf("expected pay-api target 127.0.0.1:8892, got %q", payAPIUpstream.Http.Target)
	}
	assertGatewayMappingExists(t, payAPIUpstream, "/pay/common/pay")
	assertGatewayMappingExists(t, payAPIUpstream, "/pay/detail")
	assertGatewayMappingExists(t, payAPIUpstream, "/pay/refund")

	programAPIUpstream := findGatewayUpstream(t, c.Upstreams, "program-api")
	assertGatewayMappingMissing(t, programAPIUpstream, "/program/seat/freeze")
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
	if c.Timeout <= orderAPIUpstream.Http.Timeout {
		t.Fatalf("expected gateway timeout > order-api timeout, got gateway=%d order-api=%d", c.Timeout, orderAPIUpstream.Http.Timeout)
	}
	if orderAPIUpstream.Http.Timeout != 10000 {
		t.Fatalf("expected order-api timeout 10000, got %d", orderAPIUpstream.Http.Timeout)
	}
	assertGatewayMappingExists(t, orderAPIUpstream, "/order/account/order/count")
	assertGatewayMappingExists(t, orderAPIUpstream, "/order/get/cache")

	payAPIUpstream := findGatewayUpstream(t, c.Upstreams, "pay-api")
	if payAPIUpstream.Http == nil {
		t.Fatalf("expected pay-api http upstream to be configured")
	}
	if payAPIUpstream.Http.Target != "127.0.0.1:8892" {
		t.Fatalf("expected pay-api target 127.0.0.1:8892, got %q", payAPIUpstream.Http.Target)
	}
	assertGatewayMappingExists(t, payAPIUpstream, "/pay/common/pay")
	assertGatewayMappingExists(t, payAPIUpstream, "/pay/detail")
	assertGatewayMappingExists(t, payAPIUpstream, "/pay/refund")

	programAPIUpstream := findGatewayUpstream(t, c.Upstreams, "program-api")
	assertGatewayMappingMissing(t, programAPIUpstream, "/program/seat/freeze")
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

func assertGatewayMappingExists(t *testing.T, upstream gateway.Upstream, path string) {
	t.Helper()

	for _, mapping := range upstream.Mappings {
		if mapping.Path == path {
			return
		}
	}

	t.Fatalf("expected upstream %q to contain mapping %q", upstream.Name, path)
}

func assertGatewayMappingMissing(t *testing.T, upstream gateway.Upstream, path string) {
	t.Helper()

	for _, mapping := range upstream.Mappings {
		if mapping.Path == path {
			t.Fatalf("expected upstream %q not to contain mapping %q", upstream.Name, path)
		}
	}
}
