package rush

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPurchaseTokenRoundTrip(t *testing.T) {
	codec, err := NewPurchaseTokenCodec("test-secret", 2*time.Minute)
	if err != nil {
		t.Fatalf("NewPurchaseTokenCodec returned error: %v", err)
	}
	base := time.Date(2026, time.April, 5, 18, 0, 0, 0, time.UTC)
	codec.now = func() time.Time { return base }

	token, err := codec.Issue(PurchaseTokenClaims{
		OrderNumber:      91001,
		UserID:           3001,
		ProgramID:        10001,
		ShowTimeID:       20001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701, 702},
		SaleWindowEndAt:  base.Add(30 * time.Minute).Unix(),
		ShowEndAt:        base.Add(2 * time.Hour).Unix(),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	claims, err := codec.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.OrderNumber != 91001 || claims.UserID != 3001 || claims.ProgramID != 10001 || claims.ShowTimeID != 20001 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.TicketCount != 2 {
		t.Fatalf("expected normalized ticket count, got %+v", claims)
	}
	if claims.SaleWindowEndAt != base.Add(30*time.Minute).Unix() || claims.ShowEndAt != base.Add(2*time.Hour).Unix() {
		t.Fatalf("unexpected window claims: %+v", claims)
	}
	assertTokenPayloadHasNoGeneration(t, token)
}

func TestPurchaseTokenRejectsTamperedPayload(t *testing.T) {
	codec, err := NewPurchaseTokenCodec("test-secret", 2*time.Minute)
	if err != nil {
		t.Fatalf("NewPurchaseTokenCodec returned error: %v", err)
	}

	token, err := codec.Issue(PurchaseTokenClaims{
		OrderNumber:      91001,
		UserID:           3001,
		ProgramID:        10001,
		ShowTimeID:       20001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701},
	})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	tampered := token + "tampered"
	if _, err := codec.Verify(tampered); err != ErrInvalidPurchaseToken {
		t.Fatalf("expected ErrInvalidPurchaseToken, got %v", err)
	}
}

func TestPurchaseTokenRejectsExpiredToken(t *testing.T) {
	codec, err := NewPurchaseTokenCodec("test-secret", time.Minute)
	if err != nil {
		t.Fatalf("NewPurchaseTokenCodec returned error: %v", err)
	}
	base := time.Date(2026, time.April, 5, 18, 0, 0, 0, time.UTC)
	codec.now = func() time.Time { return base }

	token, err := codec.Issue(PurchaseTokenClaims{
		OrderNumber:      91001,
		UserID:           3001,
		ProgramID:        10001,
		ShowTimeID:       20001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701},
	})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	codec.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, err := codec.Verify(token); err != ErrExpiredPurchaseToken {
		t.Fatalf("expected ErrExpiredPurchaseToken, got %v", err)
	}
}

func TestPurchaseTokenVerifyReturnsNormalizedClaims(t *testing.T) {
	codec, err := NewPurchaseTokenCodec("test-secret", 2*time.Minute)
	if err != nil {
		t.Fatalf("NewPurchaseTokenCodec returned error: %v", err)
	}
	base := time.Date(2026, time.April, 5, 18, 0, 0, 0, time.UTC)
	codec.now = func() time.Time { return base }

	payload, err := json.Marshal(PurchaseTokenClaims{
		OrderNumber:      91001,
		UserID:           3001,
		ProgramID:        10001,
		ShowTimeID:       20001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701, 702},
		ExpireAt:         base.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	token := fmt.Sprintf(
		"%s.%s.%s",
		purchaseTokenVersion,
		base64.RawURLEncoding.EncodeToString(payload),
		base64.RawURLEncoding.EncodeToString(codec.sign(payload)),
	)

	claims, err := codec.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.TicketCount != 2 {
		t.Fatalf("expected normalized ticket count 2, got %d", claims.TicketCount)
	}
	if claims.ShowTimeID != 20001 {
		t.Fatalf("expected normalized show time claims without generation, got %+v", claims)
	}
}

func TestPurchaseTokenRoundTripDoesNotEmitTokenFingerprint(t *testing.T) {
	codec, err := NewPurchaseTokenCodec("test-secret", 2*time.Minute)
	if err != nil {
		t.Fatalf("NewPurchaseTokenCodec returned error: %v", err)
	}

	token, err := codec.Issue(PurchaseTokenClaims{
		OrderNumber:      91001,
		UserID:           3001,
		ProgramID:        10001,
		ShowTimeID:       20001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701, 702},
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("Issue returned error: %v", err)
	}

	assertTokenPayloadHasNoFingerprint(t, token)
}

func TestPurchaseTokenVerifyIgnoresLegacyGenerationPayload(t *testing.T) {
	codec, err := NewPurchaseTokenCodec("test-secret", 2*time.Minute)
	if err != nil {
		t.Fatalf("NewPurchaseTokenCodec returned error: %v", err)
	}
	base := time.Date(2026, time.April, 5, 18, 0, 0, 0, time.UTC)
	codec.now = func() time.Time { return base }

	payload := []byte(`{"orderNumber":91001,"userId":3001,"programId":10001,"showTimeId":20001,"ticketCategoryId":40001,"ticketUserIds":[701,702],"ticketCount":2,"generation":"g-20001","expireAt":1775412060}`)
	token := fmt.Sprintf(
		"%s.%s.%s",
		purchaseTokenVersion,
		base64.RawURLEncoding.EncodeToString(payload),
		base64.RawURLEncoding.EncodeToString(codec.sign(payload)),
	)

	claims, err := codec.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if claims.ShowTimeID != 20001 || claims.TicketCount != 2 {
		t.Fatalf("expected legacy payload to remain decodable, got %+v", claims)
	}
}

func assertTokenPayloadHasNoGeneration(t *testing.T, token string) {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token format: %q", token)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if _, ok := decoded["generation"]; ok {
		t.Fatalf("expected token payload without generation, got %s", string(payload))
	}
}

func assertTokenPayloadHasNoFingerprint(t *testing.T, token string) {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("unexpected token format: %q", token)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("DecodeString returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if _, ok := decoded["tokenFingerprint"]; ok {
		t.Fatalf("expected token payload without tokenFingerprint, got %s", string(payload))
	}
}
