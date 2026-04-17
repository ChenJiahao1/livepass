package xmiddleware_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"livepass/pkg/xerr"
	"livepass/pkg/xmiddleware"
)

func TestAttachGatewayIdentityHeadersAndAuthenticateGatewayIdentity(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)

	if err := xmiddleware.AttachGatewayIdentityHeaders(req.Header, 3001, "gateway-internal-secret"); err != nil {
		t.Fatalf("AttachGatewayIdentityHeaders returned error: %v", err)
	}

	userID, err := xmiddleware.AuthenticateGatewayIdentity(req, "gateway-internal-secret", time.Minute)
	if err != nil {
		t.Fatalf("AuthenticateGatewayIdentity returned error: %v", err)
	}
	if userID != 3001 {
		t.Fatalf("expected user id 3001, got %d", userID)
	}
}

func TestAuthenticateGatewayIdentityRejectsExpiredTimestamp(t *testing.T) {
	timestamp := time.Now().Add(-10 * time.Minute).Unix()
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("X-User-Id", "3001")
	req.Header.Set("X-Gateway-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Gateway-Signature", signGatewayIdentity(t, 3001, timestamp, "gateway-internal-secret"))

	_, err := xmiddleware.AuthenticateGatewayIdentity(req, "gateway-internal-secret", time.Minute)
	if !errors.Is(err, xerr.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAuthenticateGatewayIdentityRejectsInvalidSignature(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("X-User-Id", "3001")
	req.Header.Set("X-Gateway-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Gateway-Signature", "bad-signature")

	_, err := xmiddleware.AuthenticateGatewayIdentity(req, "gateway-internal-secret", time.Minute)
	if !errors.Is(err, xerr.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func signGatewayIdentity(t *testing.T, userID int64, timestamp int64, secret string) string {
	t.Helper()

	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write([]byte(strconv.FormatInt(userID, 10) + ":" + strconv.FormatInt(timestamp, 10)))
	if err != nil {
		t.Fatalf("write hmac payload: %v", err)
	}

	return hex.EncodeToString(mac.Sum(nil))
}
