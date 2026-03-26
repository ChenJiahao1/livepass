package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"damai-go/pkg/xmiddleware"
	"damai-go/services/gateway-api/internal/middleware"
	"damai-go/services/gateway-api/tests/testkit"
)

func TestAuthMiddlewareRejectsOrderRequestWithoutAuthorization(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("X-Channel-Code", "0001")

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

func TestAuthMiddlewareRejectsOrderRequestWithoutChannelCode(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "Bearer "+testkit.MustCreateToken(t, 3001, "secret-0001"))

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
		t.Fatalf("expected non-200 status for missing channel code, got %d", recorder.Code)
	}
}

func TestAuthMiddlewarePassesOrderRequestWithValidToken(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "Bearer "+testkit.MustCreateToken(t, 3001, "secret-0001"))
	req.Header.Set("X-Channel-Code", "0001")

	recorder := httptest.NewRecorder()
	var called bool
	var gotUserID int64

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := xmiddleware.UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in request context")
		}
		gotUserID = userID
		w.WriteHeader(http.StatusNoContent)
	})(recorder, req)

	if !called {
		t.Fatal("expected downstream handler called")
	}
	if gotUserID != 3001 {
		t.Fatalf("expected user id 3001, got %d", gotUserID)
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareRejectsAgentRequestWithoutAuthorization(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/agent/chat", nil)
	req.Header.Set("X-Channel-Code", "0001")

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

	m := middleware.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/pay/detail", nil)
	req.Header.Set("X-Channel-Code", "0001")

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

func TestAuthMiddlewarePassesPayRequestWithValidToken(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/pay/detail", nil)
	req.Header.Set("Authorization", "Bearer "+testkit.MustCreateToken(t, 3001, "secret-0001"))
	req.Header.Set("X-Channel-Code", "0001")

	recorder := httptest.NewRecorder()
	var called bool
	var gotUserID int64
	var gotUserHeader string

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := xmiddleware.UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in request context")
		}
		gotUserID = userID
		gotUserHeader = r.Header.Get("X-User-Id")
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
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestAuthMiddlewarePassesAgentRequestWithValidTokenAndInjectsUserHeader(t *testing.T) {
	t.Parallel()

	m := middleware.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/agent/chat", nil)
	req.Header.Set("Authorization", "Bearer "+testkit.MustCreateToken(t, 3001, "secret-0001"))
	req.Header.Set("X-Channel-Code", "0001")

	recorder := httptest.NewRecorder()
	var called bool
	var gotUserID int64
	var gotUserHeader string

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, ok := xmiddleware.UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in request context")
		}
		gotUserID = userID
		gotUserHeader = r.Header.Get("X-User-Id")
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
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

func TestAuthMiddlewareSkipsUserRoute(t *testing.T) {
	t.Parallel()
	assertRouteBypassesAuth(t, "/user/login")
}

func TestAuthMiddlewareSkipsProgramRoute(t *testing.T) {
	t.Parallel()
	assertRouteBypassesAuth(t, "/program/page")
}

func assertRouteBypassesAuth(t *testing.T, path string) {
	t.Helper()

	m := middleware.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, path, nil)
	recorder := httptest.NewRecorder()
	var called bool

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusAccepted)
	})(recorder, req)

	if !called {
		t.Fatalf("expected downstream handler called for %s", path)
	}
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("expected status 202 for %s, got %d", path, recorder.Code)
	}
}
