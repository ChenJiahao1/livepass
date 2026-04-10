package rush

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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
		Generation:       BuildRushGeneration(20001),
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
	if claims.TicketCount != 2 || claims.TokenFingerprint == "" || claims.Generation != BuildRushGeneration(20001) {
		t.Fatalf("expected ticket count and fingerprint, got %+v", claims)
	}
	if claims.SaleWindowEndAt != base.Add(30*time.Minute).Unix() || claims.ShowEndAt != base.Add(2*time.Hour).Unix() {
		t.Fatalf("unexpected window claims: %+v", claims)
	}
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
		Generation:       BuildRushGeneration(20001),
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
	if claims.TokenFingerprint == "" {
		t.Fatalf("expected normalized token fingerprint, got empty")
	}
	if claims.Generation != BuildRushGeneration(20001) || claims.ShowTimeID != 20001 {
		t.Fatalf("expected normalized show time generation claims, got %+v", claims)
	}
}

func TestPurchaseTokenFingerprintDiffersAcrossOrderNumbers(t *testing.T) {
	codec, err := NewPurchaseTokenCodec("test-secret", 2*time.Minute)
	if err != nil {
		t.Fatalf("NewPurchaseTokenCodec returned error: %v", err)
	}

	firstToken, err := codec.Issue(PurchaseTokenClaims{
		OrderNumber:      91001,
		UserID:           3001,
		ProgramID:        10001,
		ShowTimeID:       20001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701, 702},
		Generation:       BuildRushGeneration(20001),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("Issue(first) returned error: %v", err)
	}
	secondToken, err := codec.Issue(PurchaseTokenClaims{
		OrderNumber:      91002,
		UserID:           3001,
		ProgramID:        10001,
		ShowTimeID:       20001,
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{702, 701},
		Generation:       BuildRushGeneration(20001),
		DistributionMode: "express",
		TakeTicketMode:   "paper",
	})
	if err != nil {
		t.Fatalf("Issue(second) returned error: %v", err)
	}

	firstClaims, err := codec.Verify(firstToken)
	if err != nil {
		t.Fatalf("Verify(first) returned error: %v", err)
	}
	secondClaims, err := codec.Verify(secondToken)
	if err != nil {
		t.Fatalf("Verify(second) returned error: %v", err)
	}

	if firstClaims.TokenFingerprint == secondClaims.TokenFingerprint {
		t.Fatalf("expected different fingerprints for different order numbers, got %q", firstClaims.TokenFingerprint)
	}
}
