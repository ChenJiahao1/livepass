package integration_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"livepass/services/gateway-api/tests/testkit"
)

func TestGatewayForwardsAgentRunsWithInjectedUserHeader(t *testing.T) {
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
	var gotAuthorization string
	agentsAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUserHeader = r.Header.Get("X-User-Id")
		gotAuthorization = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"thread":{"id":"thr_01","activeRunId":"run_01"},"run":{"id":"run_01","threadId":"thr_01","outputMessageId":"msg_01","status":"queued"},"inputMessage":{"id":"msg_user_01","runId":"run_01","metadata":{},"content":[{"type":"text","text":"帮我查订单"}]},"outputMessage":{"id":"msg_01","threadId":"thr_01","runId":"run_01","role":"assistant","status":"streaming","content":[]}}`))
	}))
	defer agentsAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000, agentsAPI.URL))
	defer server.Stop()

	headers := map[string]string{
		"Authorization": "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
	}
	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/agent/runs", headers, bytes.NewBufferString(`{"threadId":"thr_01","input":{"content":[{"type":"text","text":"帮我查订单"}]},"metadata":{}}`))
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if gotPath != "/agent/runs" {
		t.Fatalf("expected upstream path /agent/runs, got %q", gotPath)
	}
	if gotAuthorization != "" {
		t.Fatalf("expected Authorization stripped before reaching agents upstream, got %q", gotAuthorization)
	}
	if gotUserHeader != "3001" {
		t.Fatalf("expected X-User-Id 3001, got %q", gotUserHeader)
	}
	if string(body) != `{"thread":{"id":"thr_01","activeRunId":"run_01"},"run":{"id":"run_01","threadId":"thr_01","outputMessageId":"msg_01","status":"queued"},"inputMessage":{"id":"msg_user_01","runId":"run_01","metadata":{},"content":[{"type":"text","text":"帮我查订单"}]},"outputMessage":{"id":"msg_01","threadId":"thr_01","runId":"run_01","role":"assistant","status":"streaming","content":[]}}` {
		t.Fatalf("expected agents body, got %s", string(body))
	}
}

func TestGatewayDoesNotForwardLegacyAgentMessageRoute(t *testing.T) {
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

	headers := map[string]string{
		"Authorization": "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
	}
	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodPost, "/agent/threads/thr_01/messages", headers, bytes.NewBufferString(`{}`))
	defer resp.Body.Close()

	if called {
		t.Fatal("expected legacy route not to reach agents upstream")
	}
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 404 or 405, got %d", resp.StatusCode)
	}
}

func TestGatewayForwardsAgentRunEventsAfterQuery(t *testing.T) {
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
	var gotQuery string
	var gotUserHeader string
	agentsAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotUserHeader = r.Header.Get("X-User-Id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("id: 13\nevent: agent.run.event\ndata: {\"schemaVersion\":\"2026-04-16\",\"sequenceNo\":13,\"type\":\"run.updated\"}\n\n"))
	}))
	defer agentsAPI.Close()

	server, baseURL := testkit.StartTestGateway(t, testkit.NewTestConfig(t, userAPI.URL, programAPI.URL, orderAPI.URL, payAPI.URL, 1000, agentsAPI.URL))
	defer server.Stop()

	headers := map[string]string{
		"Authorization": "Bearer " + testkit.MustCreateToken(t, 3001, "secret-0001"),
	}
	resp := testkit.DoGatewayRequest(t, baseURL, http.MethodGet, "/agent/runs/run_01/events?after=12", headers, nil)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if gotPath != "/agent/runs/run_01/events" {
		t.Fatalf("expected upstream path /agent/runs/run_01/events, got %q", gotPath)
	}
	if gotQuery != "after=12" {
		t.Fatalf("expected raw query after=12, got %q", gotQuery)
	}
	if gotUserHeader != "3001" {
		t.Fatalf("expected X-User-Id 3001, got %q", gotUserHeader)
	}
	if string(body) != "id: 13\nevent: agent.run.event\ndata: {\"schemaVersion\":\"2026-04-16\",\"sequenceNo\":13,\"type\":\"run.updated\"}\n\n" {
		t.Fatalf("expected stream body preserved, got %q", string(body))
	}
}
