package xmiddleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"livepass/pkg/xerr"
	"livepass/pkg/xjwt"
	"livepass/pkg/xmiddleware"
)

func TestAuthenticateReturnsUserIDFromBearerToken(t *testing.T) {
	token, err := xjwt.CreateToken(3001, "secret-0001", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	userID, err := xmiddleware.Authenticate(req, "secret-0001")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if userID != 3001 {
		t.Fatalf("expected user id 3001, got %d", userID)
	}
}

func TestAuthenticateRejectsMissingBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "invalid-token")

	_, err := xmiddleware.Authenticate(req, "secret-0001")
	if !errors.Is(err, xerr.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestAuthenticateRejectsInvalidSignature(t *testing.T) {
	token, err := xjwt.CreateToken(3001, "secret-0001", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/order/create", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err = xmiddleware.Authenticate(req, "secret-0002")
	if !errors.Is(err, xerr.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestWithUserIDRoundTrip(t *testing.T) {
	ctx := xmiddleware.WithUserID(context.Background(), 3001)
	userID, ok := xmiddleware.UserIDFromContext(ctx)
	if !ok {
		t.Fatalf("expected user id in context")
	}
	if userID != 3001 {
		t.Fatalf("expected user id 3001, got %d", userID)
	}
}
