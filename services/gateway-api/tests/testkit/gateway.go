package testkit

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/gateway"

	"damai-go/pkg/xjwt"
	"damai-go/services/gateway-api/internal/config"
	"damai-go/services/gateway-api/internal/middleware"
)

var gatewayStartMu sync.Mutex

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
	c.Auth.AccessSecret = "secret-0001"
	c.InternalAuth.Secret = "gateway-internal-secret"
	c.Upstreams = []gateway.Upstream{
		{
			Name: "user-api",
			Http: &gateway.HttpClientConf{
				Target:  mustHostFromURL(t, userTarget),
				Timeout: timeout,
			},
			Mappings: []gateway.RouteMapping{
				{Method: http.MethodPost, Path: "/user/register"},
				{Method: http.MethodPost, Path: "/user/exist"},
				{Method: http.MethodPost, Path: "/user/get/id"},
				{Method: http.MethodPost, Path: "/user/get/mobile"},
				{Method: http.MethodPost, Path: "/user/get/user/ticket/list"},
				{Method: http.MethodPost, Path: "/user/login"},
				{Method: http.MethodPost, Path: "/user/logout"},
				{Method: http.MethodPost, Path: "/user/update"},
				{Method: http.MethodPost, Path: "/user/update/email"},
				{Method: http.MethodPost, Path: "/user/update/mobile"},
				{Method: http.MethodPost, Path: "/user/update/password"},
				{Method: http.MethodPost, Path: "/user/authentication"},
				{Method: http.MethodPost, Path: "/ticket/user/list"},
				{Method: http.MethodPost, Path: "/ticket/user/add"},
				{Method: http.MethodPost, Path: "/ticket/user/delete"},
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
				{Method: http.MethodPost, Path: "/agent/threads"},
				{Method: http.MethodGet, Path: "/agent/threads"},
				{Method: http.MethodGet, Path: "/agent/threads/:threadId"},
				{Method: http.MethodPatch, Path: "/agent/threads/:threadId"},
				{Method: http.MethodGet, Path: "/agent/threads/:threadId/messages"},
				{Method: http.MethodPost, Path: "/agent/runs"},
				{Method: http.MethodGet, Path: "/agent/runs/:runId"},
				{Method: http.MethodGet, Path: "/agent/runs/:runId/stream"},
				{Method: http.MethodPost, Path: "/agent/runs/:runId/tool-calls/:toolCallId/resume"},
				{Method: http.MethodPost, Path: "/agent/runs/:runId/cancel"},
			},
		})
	}

	return c
}

func StartTestGateway(t *testing.T, c config.Config) (*gateway.Server, string) {
	t.Helper()

	gatewayStartMu.Lock()
	defer gatewayStartMu.Unlock()

	if c.Port == 0 {
		c.Port = freePort(t)
	}

	server := gateway.MustNewServer(c.GatewayConf)
	server.Use(middleware.NewCorsMiddleware(c.Cors).Handle)
	server.Use(middleware.NewAuthMiddleware(c.Auth.AccessSecret, c.InternalAuth.Secret).Handle)
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
