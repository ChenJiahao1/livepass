package integration_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"damai-go/services/gateway-api/tests/testkit"
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

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
	defer server.Stop()

	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/user/login", nil, nil)
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

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
	defer server.Stop()

	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/program/page", nil, bytes.NewBufferString(`{}`))
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

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
	defer server.Stop()

	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/order/create", nil, bytes.NewBufferString(`{}`))
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

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
	defer server.Stop()

	headers := map[string]string{
		"Authorization":  "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
		"X-Channel-Code": "0001",
	}
	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/order/create", headers, bytes.NewBufferString(`{}`))
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

func TestGatewayForwardsAuthorizedRefundOrderRequest(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.NotFoundHandler())
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	var gotPath string
	orderAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"order-refund"}`))
	}))
	defer orderAPI.Close()

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
	defer server.Stop()

	headers := map[string]string{
		"Authorization":  "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
		"X-Channel-Code": "0001",
	}
	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/order/refund", headers, bytes.NewBufferString(`{"orderNumber":91001}`))
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if gotPath != "/order/refund" {
		t.Fatalf("expected upstream path /order/refund, got %q", gotPath)
	}
	if string(body) != `{"service":"order-refund"}` {
		t.Fatalf("expected refund body, got %s", string(body))
	}
}

func TestGatewayForwardsAuthorizedOrderManagementRoutes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		path string
		body string
	}{
		{
			name: "account order count",
			path: "/order/account/order/count",
			body: `{"service":"order-account-count"}`,
		},
		{
			name: "get order cache",
			path: "/order/get/cache",
			body: `{"service":"order-cache"}`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			userAPI := httptest.NewServer(http.NotFoundHandler())
			defer userAPI.Close()

			programAPI := httptest.NewServer(http.NotFoundHandler())
			defer programAPI.Close()

			var gotPath string
			orderAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer orderAPI.Close()

			payAPI := httptest.NewServer(http.NotFoundHandler())
			defer payAPI.Close()

			server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
			defer server.Stop()

			headers := map[string]string{
				"Authorization":  "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
				"X-Channel-Code": "0001",
			}
			resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, tc.path, headers, bytes.NewBufferString(`{}`))
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}
			if gotPath != tc.path {
				t.Fatalf("expected upstream path %s, got %q", tc.path, gotPath)
			}
			if string(body) != tc.body {
				t.Fatalf("expected body %s, got %s", tc.body, string(body))
			}
		})
	}
}

func TestGatewayForwardsAuthorizedPayRequests(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		path string
		body string
	}{
		{
			name: "common pay",
			path: "/pay/common/pay",
			body: `{"service":"pay-common"}`,
		},
		{
			name: "pay detail",
			path: "/pay/detail",
			body: `{"service":"pay-detail"}`,
		},
		{
			name: "pay refund",
			path: "/pay/refund",
			body: `{"service":"pay-refund"}`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			userAPI := httptest.NewServer(http.NotFoundHandler())
			defer userAPI.Close()

			programAPI := httptest.NewServer(http.NotFoundHandler())
			defer programAPI.Close()

			orderAPI := httptest.NewServer(http.NotFoundHandler())
			defer orderAPI.Close()

			var gotPath string
			payAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer payAPI.Close()

			server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
			defer server.Stop()

			headers := map[string]string{
				"Authorization":  "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
				"X-Channel-Code": "0001",
			}
			resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, tc.path, headers, bytes.NewBufferString(`{}`))
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200, got %d", resp.StatusCode)
			}
			if gotPath != tc.path {
				t.Fatalf("expected upstream path %s, got %q", tc.path, gotPath)
			}
			if string(body) != tc.body {
				t.Fatalf("expected body %s, got %s", tc.body, string(body))
			}
		})
	}
}

func TestGatewayBlocksUnauthorizedAgentRequest(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.NotFoundHandler())
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	orderAPI := httptest.NewServer(http.NotFoundHandler())
	defer orderAPI.Close()

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	var called bool
	agentsAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer agentsAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000, agentsAPI.URL))
	defer server.Stop()

	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/agent/chat", nil, bytes.NewBufferString(`{"message":"hi"}`))
	defer resp.Body.Close()

	if called {
		t.Fatal("expected unauthorized agent request to be blocked before reaching agents upstream")
	}
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected non-200 status for unauthorized agent request, got %d", resp.StatusCode)
	}
}

func TestGatewayForwardsAuthorizedAgentRequestWithUserHeader(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.NotFoundHandler())
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	orderAPI := httptest.NewServer(http.NotFoundHandler())
	defer orderAPI.Close()

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	var gotPath string
	var gotUserHeader string
	agentsAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUserHeader = r.Header.Get("X-User-Id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"service":"agents"}`))
	}))
	defer agentsAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000, agentsAPI.URL))
	defer server.Stop()

	headers := map[string]string{
		"Authorization":  "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
		"X-Channel-Code": "0001",
	}
	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/agent/chat", headers, bytes.NewBufferString(`{"message":"hi"}`))
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if gotPath != "/agent/chat" {
		t.Fatalf("expected upstream path /agent/chat, got %q", gotPath)
	}
	if gotUserHeader != "3001" {
		t.Fatalf("expected X-User-Id 3001, got %q", gotUserHeader)
	}
	if string(body) != `{"service":"agents"}` {
		t.Fatalf("expected agents body, got %s", string(body))
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

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
	defer server.Stop()

	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/user/login", nil, bytes.NewBufferString(`{}`))
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

func TestGatewayPreservesOrderAPIStatusCodes(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "too many requests",
			statusCode: http.StatusTooManyRequests,
			body:       `{"message":"提交频繁，请稍后重试"}`,
		},
		{
			name:       "service unavailable",
			statusCode: http.StatusServiceUnavailable,
			body:       `{"message":"repeat guard unavailable"}`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			userAPI := httptest.NewServer(http.NotFoundHandler())
			defer userAPI.Close()

			programAPI := httptest.NewServer(http.NotFoundHandler())
			defer programAPI.Close()

			orderAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer orderAPI.Close()

			payAPI := httptest.NewServer(http.NotFoundHandler())
			defer payAPI.Close()

			server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000))
			defer server.Stop()

			headers := map[string]string{
				"Authorization":  "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
				"X-Channel-Code": "0001",
			}
			resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/order/create", headers, bytes.NewBufferString(`{}`))
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}

			if resp.StatusCode != tc.statusCode {
				t.Fatalf("expected status %d, got %d", tc.statusCode, resp.StatusCode)
			}
			if string(body) != tc.body {
				t.Fatalf("expected body %s, got %s", tc.body, string(body))
			}
		})
	}
}

func TestGatewayReturnsErrorWhenUpstreamTimeout(t *testing.T) {
	t.Parallel()

	userAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer userAPI.Close()

	programAPI := httptest.NewServer(http.NotFoundHandler())
	defer programAPI.Close()

	orderAPI := httptest.NewServer(http.NotFoundHandler())
	defer orderAPI.Close()

	payAPI := httptest.NewServer(http.NotFoundHandler())
	defer payAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 10))
	defer server.Stop()

	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/user/login", nil, bytes.NewBufferString(`{}`))
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected timeout request to return non-200 status, got %d", resp.StatusCode)
	}
}
