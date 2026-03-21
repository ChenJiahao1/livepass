package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"damai-go/pkg/xjwt"
	"damai-go/pkg/xmiddleware"
)

func TestAuthMiddlewareRejectsOrderRequestWithoutAuthorization(t *testing.T) {
	t.Parallel()

	m := NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
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

	m := NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "Bearer "+mustCreateToken(t, 3001, "secret-0001"))

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

	m := NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "Bearer "+mustCreateToken(t, 3001, "secret-0001"))
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

	m := NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
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

func mustCreateToken(t *testing.T, userID int64, secret string) string {
	t.Helper()

	token, err := xjwt.CreateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	return token
}
