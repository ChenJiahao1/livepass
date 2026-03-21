package integration_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"damai-go/pkg/xjwt"
	"damai-go/pkg/xmiddleware"
	middlewarepkg "damai-go/services/order-api/internal/middleware"
)

func TestAuthMiddlewareInjectsUserIDIntoContext(t *testing.T) {
	token, err := xjwt.CreateToken(3001, "secret-0001", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	m := middlewarepkg.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Channel-Code", "0001")

	recorder := httptest.NewRecorder()
	var gotUserID int64
	var called bool

	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
		var ok bool
		gotUserID, ok = xmiddleware.UserIDFromContext(r.Context())
		if !ok {
			t.Fatalf("expected user id in request context")
		}
		w.WriteHeader(http.StatusOK)
	})(recorder, req)

	if !called {
		t.Fatalf("expected downstream handler called")
	}
	if gotUserID != 3001 {
		t.Fatalf("expected user id 3001, got %d", gotUserID)
	}
}

func TestAuthMiddlewareRejectsInvalidHeader(t *testing.T) {
	m := middlewarepkg.NewAuthMiddleware("X-Channel-Code", map[string]string{"0001": "secret-0001"})
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "invalid-token")
	req.Header.Set("X-Channel-Code", "0001")

	recorder := httptest.NewRecorder()
	var called bool
	m.Handle(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})(recorder, req)

	if called {
		t.Fatalf("expected downstream handler not called")
	}
	if recorder.Code == http.StatusOK {
		t.Fatalf("expected auth middleware to reject request")
	}
}
