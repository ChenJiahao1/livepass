package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"livepass/pkg/xmiddleware"
	"livepass/services/gateway-api/internal/middleware"
	"livepass/services/gateway-api/tests/testkit"
)

func TestAuthMiddlewareRejectsOrderRequestWithoutAuthorization(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)

	recorder := httptest.NewRecorder()
	var called bool

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})(recorder, req)

	if called {
		t.Fatal("expected downstream handler not called")
	}
	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for unauthorized request, got %d", recorder.Code)
	}
}

func TestAuthMiddlewarePassesOrderRequestWithValidToken(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "Bearer "+testkit.MustCreateToken(t, 3001, "secret-0001"))
	req.Header.Set("X-User-Id", "9999")
	req.Header.Set("X-Gateway-Timestamp", "1")
	req.Header.Set("X-Gateway-Signature", "spoofed")

	recorder := httptest.NewRecorder()
	var called bool
	var gotUserID int64
	var gotAuthorization string
	var gotUserHeader string
	var gotTimestamp string
	var gotSignature string

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := xmiddleware.UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in request context")
		}
		gotUserID = userID
		gotAuthorization = r.Header.Get("Authorization")
		gotUserHeader = r.Header.Get("X-User-Id")
		gotTimestamp = r.Header.Get("X-Gateway-Timestamp")
		gotSignature = r.Header.Get("X-Gateway-Signature")
		w.WriteHeader(http.StatusNoContent)
	})(recorder, req)

	if !called {
		t.Fatal("expected downstream handler called")
	}
	if gotUserID != 3001 {
		t.Fatalf("expected user id 3001, got %d", gotUserID)
	}
	if gotAuthorization != "" {
		t.Fatalf("expected Authorization stripped before forwarding, got %q", gotAuthorization)
	}
	if gotUserHeader != "3001" {
		t.Fatalf("expected X-User-Id 3001, got %q", gotUserHeader)
	}
	if gotTimestamp == "" {
		t.Fatal("expected X-Gateway-Timestamp to be injected")
	}
	if gotSignature == "" || gotSignature == "spoofed" {
		t.Fatalf("expected X-Gateway-Signature to be injected, got %q", gotSignature)
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareRejectsAgentRequestWithoutAuthorization(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, "/agent/runs", nil)

	recorder := httptest.NewRecorder()
	var called bool

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})(recorder, req)

	if called {
		t.Fatal("expected downstream handler not called")
	}
	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for unauthorized agent request, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareRejectsPayRequestWithoutAuthorization(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, "/pay/detail", nil)

	recorder := httptest.NewRecorder()
	var called bool

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})(recorder, req)

	if called {
		t.Fatal("expected downstream handler not called")
	}
	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for unauthorized pay request, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareRejectsProtectedUserRequestWithoutAuthorization(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, "/user/update", nil)

	recorder := httptest.NewRecorder()
	var called bool

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})(recorder, req)

	if called {
		t.Fatal("expected downstream handler not called")
	}
	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for unauthorized user request, got %d", recorder.Code)
	}
}

func TestAuthMiddlewarePassesPayRequestWithValidToken(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, "/pay/detail", nil)
	req.Header.Set("Authorization", "Bearer "+testkit.MustCreateToken(t, 3001, "secret-0001"))

	recorder := httptest.NewRecorder()
	var called bool
	var gotUserID int64
	var gotUserHeader string
	var gotTimestamp string
	var gotSignature string

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := xmiddleware.UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in request context")
		}
		gotUserID = userID
		gotUserHeader = r.Header.Get("X-User-Id")
		gotTimestamp = r.Header.Get("X-Gateway-Timestamp")
		gotSignature = r.Header.Get("X-Gateway-Signature")
		w.WriteHeader(http.StatusNoContent)
	})(recorder, req)

	if !called {
		t.Fatal("expected downstream handler called")
	}
	if gotUserID != 3001 {
		t.Fatalf("expected user id 3001, got %d", gotUserID)
	}
	if gotUserHeader != "3001" {
		t.Fatalf("expected X-User-Id 3001, got %q", gotUserHeader)
	}
	if gotTimestamp == "" {
		t.Fatal("expected X-Gateway-Timestamp to be injected")
	}
	if gotSignature == "" {
		t.Fatal("expected X-Gateway-Signature to be injected")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestAuthMiddlewarePassesAgentRequestWithValidTokenAndInjectsUserHeader(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, "/agent/runs", nil)
	req.Header.Set("Authorization", "Bearer "+testkit.MustCreateToken(t, 3001, "secret-0001"))

	recorder := httptest.NewRecorder()
	var called bool
	var gotUserID int64
	var gotUserHeader string
	var gotTimestamp string
	var gotSignature string

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := xmiddleware.UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in request context")
		}
		gotUserID = userID
		gotUserHeader = r.Header.Get("X-User-Id")
		gotTimestamp = r.Header.Get("X-Gateway-Timestamp")
		gotSignature = r.Header.Get("X-Gateway-Signature")
		w.WriteHeader(http.StatusNoContent)
	})(recorder, req)

	if !called {
		t.Fatal("expected downstream handler called")
	}
	if gotUserID != 3001 {
		t.Fatalf("expected user id 3001, got %d", gotUserID)
	}
	if gotUserHeader != "3001" {
		t.Fatalf("expected X-User-Id 3001, got %q", gotUserHeader)
	}
	if gotTimestamp == "" {
		t.Fatal("expected X-Gateway-Timestamp to be injected")
	}
	if gotSignature == "" {
		t.Fatal("expected X-Gateway-Signature to be injected")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestAuthMiddlewarePassesPerfOrderRequestWithValidPerfHeaders(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{
		Enabled:      true,
		HeaderName:   "X-LivePass-Perf-Secret",
		HeaderSecret: "perf-secret-0001",
		UserIDHeader: "X-LivePass-Perf-User-Id",
		AllowedPaths: map[string]struct{}{
			"/order/create": {},
			"/order/poll":   {},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("X-LivePass-Perf-Secret", "perf-secret-0001")
	req.Header.Set("X-LivePass-Perf-User-Id", "7788")

	recorder := httptest.NewRecorder()
	var called bool
	var gotUserID int64
	var gotAuthorization string
	var gotUserHeader string

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := xmiddleware.UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in request context")
		}
		gotUserID = userID
		gotAuthorization = r.Header.Get("Authorization")
		gotUserHeader = r.Header.Get("X-User-Id")
		w.WriteHeader(http.StatusNoContent)
	})(recorder, req)

	if !called {
		t.Fatal("expected downstream handler called")
	}
	if gotUserID != 7788 {
		t.Fatalf("expected user id 7788, got %d", gotUserID)
	}
	if gotAuthorization != "" {
		t.Fatalf("expected Authorization stripped before forwarding, got %q", gotAuthorization)
	}
	if gotUserHeader != "7788" {
		t.Fatalf("expected X-User-Id 7788, got %q", gotUserHeader)
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareRejectsPerfOrderRequestWithInvalidPerfSecret(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{
		Enabled:      true,
		HeaderName:   "X-LivePass-Perf-Secret",
		HeaderSecret: "perf-secret-0001",
		UserIDHeader: "X-LivePass-Perf-User-Id",
		AllowedPaths: map[string]struct{}{
			"/order/create": {},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("X-LivePass-Perf-Secret", "perf-secret-invalid")
	req.Header.Set("X-LivePass-Perf-User-Id", "7788")

	recorder := httptest.NewRecorder()
	var called bool

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})(recorder, req)

	if called {
		t.Fatal("expected downstream handler not called")
	}
	if recorder.Code == http.StatusNoContent {
		t.Fatalf("expected non-204 status for invalid perf secret, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareSkipsUserRoute(t *testing.T) {
	t.Parallel()
	assertRouteBypassesAuth(t, "/user/login")
}

func TestAuthMiddlewareProtectsUserProfileQueryRoute(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, "/user/get/id", nil)

	recorder := httptest.NewRecorder()
	var called bool

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})(recorder, req)

	if called {
		t.Fatal("expected downstream handler not called")
	}
	if recorder.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for unauthorized user profile query, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareSkipsProgramRoute(t *testing.T) {
	t.Parallel()
	assertRouteBypassesAuth(t, "/program/page")
}

func assertRouteBypassesAuth(t *testing.T, path string) {
	t.Helper()

	m := middleware.NewAuthMiddleware("secret-0001", "gateway-internal-secret", middleware.PerfAuthConfig{})
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Authorization", "Bearer should-be-stripped")
	req.Header.Set("X-User-Id", "should-be-stripped")
	recorder := httptest.NewRecorder()
	var called bool
	var gotAuthorization string
	var gotUserHeader string
	var gotTimestamp string
	var gotSignature string

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotAuthorization = r.Header.Get("Authorization")
		gotUserHeader = r.Header.Get("X-User-Id")
		gotTimestamp = r.Header.Get("X-Gateway-Timestamp")
		gotSignature = r.Header.Get("X-Gateway-Signature")
		w.WriteHeader(http.StatusAccepted)
	})(recorder, req)

	if !called {
		t.Fatalf("expected downstream handler called for %s", path)
	}
	if gotAuthorization != "" {
		t.Fatalf("expected Authorization stripped for %s, got %q", path, gotAuthorization)
	}
	if gotUserHeader != "" || gotTimestamp != "" || gotSignature != "" {
		t.Fatalf("expected no internal identity headers for %s, got user=%q timestamp=%q signature=%q", path, gotUserHeader, gotTimestamp, gotSignature)
	}
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 for %s, got %d", path, recorder.Code)
	}
}
