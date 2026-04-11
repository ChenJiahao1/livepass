package testkit

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/gateway"

	"damai-go/pkg/xjwt"
	"damai-go/services/gateway-api/internal/config"
	"damai-go/services/gateway-api/internal/middleware"
)

func MustCreateToken(t *testing.T, userID int64, secret string) string {
	t.Helper()

	token, err := xjwt.CreateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	return token
}

func NewTestConfig(t *testing.T, userTarget, programTarget, orderTarget, payTarget string, timeout int64, agentsTarget ...string) config.Config {
	t.Helper()

	var c config.Config
	if err := conf.FillDefault(&c.GatewayConf); err != nil {
		t.Fatalf("fill gateway defaults: %v", err)
	}

	c.Name = "gateway-api-test"
	c.Host = "127.0.0.1"
	c.Port = freePort(t)
	c.Auth.ChannelCodeHeader = "X-Channel-Code"
	c.Auth.ChannelMap = map[string]string{"0001": "secret-0001"}
	c.Upstreams = []gateway.Upstream{
		{
			Name: "user-api",
			Http: &gateway.HttpClientConf{
				Target:  mustHostFromURL(t, userTarget),
				Timeout: timeout,
			},
			Mappings: []gateway.RouteMapping{
				{Method: http.MethodPost, Path: "/user/register"},
				{Method: http.MethodPost, Path: "/user/login"},
			},
		},
		{
			Name: "program-api",
			Http: &gateway.HttpClientConf{
				Target:  mustHostFromURL(t, programTarget),
				Timeout: timeout,
			},
			Mappings: []gateway.RouteMapping{
				{Method: http.MethodPost, Path: "/program/page"},
			},
		},
		{
			Name: "order-api",
			Http: &gateway.HttpClientConf{
				Target:  mustHostFromURL(t, orderTarget),
				Timeout: timeout,
			},
			Mappings: []gateway.RouteMapping{
				{Method: http.MethodPost, Path: "/order/account/order/count"},
				{Method: http.MethodPost, Path: "/order/create"},
				{Method: http.MethodPost, Path: "/order/get/cache"},
				{Method: http.MethodPost, Path: "/order/refund"},
			},
		},
		{
			Name: "pay-api",
			Http: &gateway.HttpClientConf{
				Target:  mustHostFromURL(t, payTarget),
				Timeout: timeout,
			},
			Mappings: []gateway.RouteMapping{
				{Method: http.MethodPost, Path: "/pay/common/pay"},
				{Method: http.MethodPost, Path: "/pay/detail"},
				{Method: http.MethodPost, Path: "/pay/refund"},
			},
		},
	}
	if len(agentsTarget) > 0 && agentsTarget[0] != "" {
		c.Upstreams = append(c.Upstreams, gateway.Upstream{
			Name: "agents-api",
			Http: &gateway.HttpClientConf{
				Target:  mustHostFromURL(t, agentsTarget[0]),
				Timeout: timeout,
			},
			Mappings: []gateway.RouteMapping{
				{Method: http.MethodPost, Path: "/agent/chat"},
			},
		})
	}

	return c
}

func StartTestGateway(t *testing.T, c config.Config) (*gateway.Server, string) {
	t.Helper()

	server := gateway.MustNewServer(c.GatewayConf)
	server.Use(middleware.NewCorsMiddleware(c.Cors).Handle)
	server.Use(middleware.NewAuthMiddleware(c.Auth.ChannelCodeHeader, c.Auth.ChannelMap).Handle)
	middleware.RegisterPreflightRoutes(server, c.Upstreams)
	go server.Start()

	baseURL := fmt.Sprintf("http://%s:%d", c.Host, c.Port)
	waitForGatewayReady(t, baseURL)
	return server, baseURL
}

func DoGatewayRequest(t *testing.T, baseURL, method, path string, headers map[string]string, body io.Reader) *http.Response {
	t.Helper()

	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		t.Fatalf("create request %s %s: %v", method, path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}

	return resp
}

func waitForGatewayReady(t *testing.T, baseURL string) {
	t.Helper()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodPost, baseURL+"/user/login", bytes.NewBufferString(`{}`))
		if err != nil {
			t.Fatalf("create readiness request: %v", err)
		}

		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("gateway did not become ready in time: %s", baseURL)
}

func mustHostFromURL(t *testing.T, rawURL string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse upstream url %q: %v", rawURL, err)
	}

	return parsed.Host
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}
