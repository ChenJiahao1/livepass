package integration_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"livepass/pkg/xmiddleware"
	middlewarepkg "livepass/services/order-api/internal/middleware"
)

func TestAuthMiddlewareInjectsUserIDIntoContext(t *testing.T) {
	m := middlewarepkg.NewAuthMiddleware("gateway-internal-secret", time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	attachInternalIdentity(req, 3001, "gateway-internal-secret", time.Now())

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

func TestAuthMiddlewareRejectsInvalidGatewaySignature(t *testing.T) {
	m := middlewarepkg.NewAuthMiddleware("gateway-internal-secret", time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("X-User-Id", "3001")
	req.Header.Set("X-Gateway-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Gateway-Signature", "invalid-signature")

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

func attachInternalIdentity(req *http.Request, userID int64, secret string, now time.Time) {
	timestamp := now.Unix()
	req.Header.Set("X-User-Id", strconv.FormatInt(userID, 10))
	req.Header.Set("X-Gateway-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Gateway-Signature", signInternalIdentity(userID, timestamp, secret))
}

func signInternalIdentity(userID int64, timestamp int64, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(timestamp, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
