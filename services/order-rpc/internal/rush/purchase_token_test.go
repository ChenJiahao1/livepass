package rush

import (
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
		TicketCategoryID: 40001,
		TicketUserIDs:    []int64{701, 702},
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
	if claims.OrderNumber != 91001 || claims.UserID != 3001 || claims.ProgramID != 10001 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims.TicketCount != 2 || claims.TokenFingerprint == "" {
		t.Fatalf("expected ticket count and fingerprint, got %+v", claims)
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
