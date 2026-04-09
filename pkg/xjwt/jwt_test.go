package xjwt_test

import (
	"errors"
	"testing"
	"time"

	"damai-go/pkg/xjwt"

	"github.com/golang-jwt/jwt/v5"
)

func TestCreateTokenAndParseTokenRoundTrip(t *testing.T) {
	token, err := xjwt.CreateToken(3001, "secret-0001", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	claims, err := xjwt.ParseToken(token, "secret-0001")
	if err != nil {
		t.Fatalf("ParseToken returned error: %v", err)
	}

	if claims.UserID != 3001 {
		t.Fatalf("expected user id 3001, got %d", claims.UserID)
	}

	if claims.IssuedAt == nil {
		t.Fatal("expected issued at to be set")
	}
	if claims.ExpiresAt == nil {
		t.Fatal("expected expires at to be set")
	}
}

func TestParseTokenRejectsInvalidSignature(t *testing.T) {
	token, err := xjwt.CreateToken(3001, "secret-0001", time.Hour)
	if err != nil {
		t.Fatalf("CreateToken returned error: %v", err)
	}

	_, err = xjwt.ParseToken(token, "secret-0002")
	if !errors.Is(err, jwt.ErrSignatureInvalid) {
		t.Fatalf("expected signature invalid error, got %v", err)
	}
}
