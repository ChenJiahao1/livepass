package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"damai-go/services/gateway-api/internal/config"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/gateway"
)

func TestGatewayForwardsUserRequestToUserAPI(t *testing.T) {
	t.Parallel()

	var gotPath string
	userAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("X-Upstream", "user-api")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"user"}`))
	}))
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	orderAPI := httptest.NewServer(http.NotFoundHandler())
	defer orderAPI.Close()

	server, baseURL := startTestGateway(t, newTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, 1000))
	defer server.Stop()

	resp := doGatewayRequest(t, baseURL, http.MethodPost, "/user/login", nil, nil)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if gotPath != "/user/login" {
		t.Fatalf("expected upstream path /user/login, got %q", gotPath)
	}
	if got := resp.Header.Get("X-Upstream"); got != "user-api" {
		t.Fatalf("expected X-Upstream user-api, got %q", got)
	}
	if string(body) != `{"service":"user"}` {
		t.Fatalf("expected user body, got %s", string(body))
	}
}

func TestGatewayForwardsProgramRequestToProgramAPI(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.NotFoundHandler())
	defer userAPI.Close()

	var gotPath string
	programAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"program"}`))
	}))
	defer programAPI.Close()

	orderAPI := httptest.NewServer(http.NotFoundHandler())
	defer orderAPI.Close()

	server, baseURL := startTestGateway(t, newTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, 1000))
	defer server.Stop()

	resp := doGatewayRequest(t, baseURL, http.MethodPost, "/program/page", nil, bytes.NewBufferString(`{}`))
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if gotPath != "/program/page" {
		t.Fatalf("expected upstream path /program/page, got %q", gotPath)
	}
	if string(body) != `{"service":"program"}` {
		t.Fatalf("expected program body, got %s", string(body))
	}
}

func TestGatewayBlocksUnauthorizedOrderRequest(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.NotFoundHandler())
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	var called bool
	orderAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer orderAPI.Close()

	server, baseURL := startTestGateway(t, newTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, 1000))
	defer server.Stop()

	resp := doGatewayRequest(t, baseURL, http.MethodPost, "/order/create", nil, bytes.NewBufferString(`{}`))
	defer resp.Body.Close()

	if called {
		t.Fatal("expected unauthorized request to be blocked before reaching order upstream")
	}
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200 status for unauthorized request, got %d", resp.StatusCode)
	}
}

func TestGatewayForwardsAuthorizedOrderRequest(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.NotFoundHandler())
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	var gotPath string
	var gotAuthorization string
	var gotChannelCode string
	orderAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthorization = r.Header.Get("Authorization")
		gotChannelCode = r.Header.Get("X-Channel-Code")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"service":"order"}`))
	}))
	defer orderAPI.Close()

	server, baseURL := startTestGateway(t, newTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, 1000))
	defer server.Stop()

	headers := map[string]string{
		"Authorization":  "Bearer " + mustCreateToken(t, 3001, "secret-0001"),
		"X-Channel-Code": "0001",
	}
	resp := doGatewayRequest(t, baseURL, http.MethodPost, "/order/create", headers, bytes.NewBufferString(`{}`))
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}
	if gotPath != "/order/create" {
		t.Fatalf("expected upstream path /order/create, got %q", gotPath)
	}
	if gotAuthorization == "" {
		t.Fatal("expected Authorization header forwarded to order upstream")
	}
	if gotChannelCode != "0001" {
		t.Fatalf("expected X-Channel-Code 0001, got %q", gotChannelCode)
	}
	if string(body) != `{"service":"order"}` {
		t.Fatalf("expected order body, got %s", string(body))
	}
}

func TestGatewayPreservesUpstreamStatusCode(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request from upstream", http.StatusBadRequest)
	}))
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	orderAPI := httptest.NewServer(http.NotFoundHandler())
	defer orderAPI.Close()

	server, baseURL := startTestGateway(t, newTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, 1000))
	defer server.Stop()

	resp := doGatewayRequest(t, baseURL, http.MethodPost, "/user/login", nil, bytes.NewBufferString(`{}`))
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
	if !bytes.Contains(body, []byte("bad request from upstream")) {
		t.Fatalf("expected upstream error body preserved, got %s", string(body))
	}
}

func TestGatewayReturnsErrorWhenUpstreamTimeout(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"user"}`))
	}))
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	orderAPI := httptest.NewServer(http.NotFoundHandler())
	defer orderAPI.Close()

	server, baseURL := startTestGateway(t, newTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, 10))
	defer server.Stop()

	resp := doGatewayRequest(t, baseURL, http.MethodPost, "/user/login", nil, bytes.NewBufferString(`{}`))
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected timeout request to return non-200 status, got %d", resp.StatusCode)
	}
}

func newTestGatewayServer(t *testing.T, c config.Config) *gateway.Server {
	t.Helper()

	return gateway.MustNewServer(
		c.GatewayConf,
		gateway.WithMiddleware(
			NewAuthMiddleware(c.Auth.ChannelCodeHeader, c.Auth.ChannelMap).Handle,
		),
	)
}

func newTestConfig(t *testing.T, userTarget, programTarget, orderTarget string, timeout int64) config.Config {
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
				{Method: http.MethodPost, Path: "/order/create"},
			},
		},
	}

	return c
}

func startTestGateway(t *testing.T, c config.Config) (*gateway.Server, string) {
	t.Helper()

	server := newTestGatewayServer(t, c)
	go server.Start()

	baseURL := fmt.Sprintf("http://%s:%d", c.Host, c.Port)
	waitForGatewayReady(t, baseURL)
	return server, baseURL
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

func doGatewayRequest(t *testing.T, baseURL, method, path string, headers map[string]string, body io.Reader) *http.Response {
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
