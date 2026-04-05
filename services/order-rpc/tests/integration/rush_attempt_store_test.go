package integration_test

import (
	"context"
	"testing"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/rush"
)

func TestAdmissionReturnsSameOrderNumberForSameFingerprint(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 30, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	viewerIDs = viewerIDs[:2]
	fingerprint := rush.BuildTokenFingerprint(userID, programID, ticketCategoryID, viewerIDs, "express", "paper")

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 6); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	first, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		TokenFingerprint: fingerprint,
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now,
	})
	if err != nil {
		t.Fatalf("Admit(first) error = %v", err)
	}
	if first.Decision != rush.AdmitDecisionAccepted || first.OrderNumber != orderNumbers[0] {
		t.Fatalf("unexpected first admission result: %+v", first)
	}

	second, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      int64(len(viewerIDs)),
		TokenFingerprint: fingerprint,
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now.Add(200 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(second) error = %v", err)
	}
	if second.Decision != rush.AdmitDecisionReused || second.OrderNumber != orderNumbers[0] {
		t.Fatalf("unexpected second admission result: %+v", second)
	}

	record, err := store.Get(ctx, orderNumbers[0])
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if record.State != rush.AttemptStatePendingPublish || record.TokenFingerprint != fingerprint {
		t.Fatalf("unexpected attempt record: %+v", record)
	}

	if _, err := store.Get(ctx, orderNumbers[1]); err == nil || err.Error() != xerr.ErrOrderNotFound.Error() {
		t.Fatalf("expected order 991002 to stay absent, got err=%v", err)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("unexpected quota snapshot: ok=%t available=%d", ok, available)
	}
}

func TestAdmissionRejectsDifferentFingerprintWhenUserInflight(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Date(2026, 4, 5, 18, 31, 0, 0, time.Local)
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 5); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}

	firstFingerprint := rush.BuildTokenFingerprint(userID, programID, ticketCategoryID, viewerIDs, "express", "paper")
	first, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[0],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: firstFingerprint,
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now,
	})
	if err != nil {
		t.Fatalf("Admit(first) error = %v", err)
	}
	if first.Decision != rush.AdmitDecisionAccepted {
		t.Fatalf("unexpected first admission result: %+v", first)
	}

	second, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumbers[1],
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: firstFingerprint + "-different",
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now.Add(100 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("Admit(second) error = %v", err)
	}
	if second.Decision != rush.AdmitDecisionRejected || second.RejectCode != rush.AdmitRejectUserInflightConflict {
		t.Fatalf("unexpected second admission result: %+v", second)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("unexpected quota snapshot after reject: ok=%t available=%d", ok, available)
	}
}
