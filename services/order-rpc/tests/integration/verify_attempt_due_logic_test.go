package integration_test

import (
	"context"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/sharding"
)

func TestVerifyAttemptDueMarksAttemptVerifyingAndReconcileReleasesMissingOrder(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	store := rebindOrderTestAttemptStore(t, svcCtx)
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Now()
	userID, programID, ticketCategoryID, viewerIDs, _ := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]
	orderNumber := sharding.BuildOrderNumber(userID, now, 1, 1)

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: rush.BuildTokenFingerprint(userID, programID, ticketCategoryID, viewerIDs, "express", "paper"),
		CommitCutoffAt:   now.Add(120 * time.Millisecond),
		UserDeadlineAt:   now.Add(-time.Second),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if err := store.MarkQueued(ctx, orderNumber, now); err != nil {
		t.Fatalf("MarkQueued() error = %v", err)
	}

	resp, err := logicpkg.NewVerifyAttemptDueLogic(ctx, svcCtx).VerifyAttemptDue(&pb.VerifyAttemptDueReq{
		OrderNumber: orderNumber,
		DueAtUnix:   now.Add(-time.Second).Unix(),
	})
	if err != nil {
		t.Fatalf("VerifyAttemptDue() error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected verify attempt due success")
	}

	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateVerifying {
		t.Fatalf("expected verifying attempt state, got %+v", record)
	}
	if record.VerifyStartedAt.IsZero() || record.LastDBProbeAt.IsZero() || record.NextDBProbeAt.IsZero() {
		t.Fatalf("expected verify timestamps to be recorded, got %+v", record)
	}
	if record.DBProbeAttempts != 1 {
		t.Fatalf("expected first DB probe attempt, got %+v", record)
	}

	time.Sleep(180 * time.Millisecond)

	reconcileResp, err := logicpkg.NewReconcileRushAttemptsLogic(ctx, svcCtx).ReconcileRushAttempts(&pb.ReconcileRushAttemptsReq{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ReconcileRushAttempts() error = %v", err)
	}
	if reconcileResp.GetReconciledCount() != 1 {
		t.Fatalf("expected one reconciled attempt, got %+v", reconcileResp)
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() after reconcile error = %v", err)
	}
	if record.State != rush.AttemptStateReleased || record.ReasonCode != rush.AttemptReasonCommitCutoffExceed {
		t.Fatalf("expected released attempt after reconcile, got %+v", record)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("expected quota restored to 4 after reconcile release, got ok=%t available=%d", ok, available)
	}
}

func TestVerifyAttemptDueCommitsProjectionWhenOrderAlreadyPersisted(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	store := rebindOrderTestAttemptStore(t, svcCtx)
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Now()
	userID, programID, ticketCategoryID, viewerIDs, _ := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]
	orderNumber := sharding.BuildOrderNumber(userID, now, 1, 2)

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: rush.BuildTokenFingerprint(userID, programID, ticketCategoryID, viewerIDs, "express", "paper"),
		CommitCutoffAt:   now.Add(5 * time.Second),
		UserDeadlineAt:   now.Add(-time.Second),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if err := store.MarkQueued(ctx, orderNumber, now); err != nil {
		t.Fatalf("MarkQueued() error = %v", err)
	}

	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8301,
			OrderNumber:     orderNumber,
			ProgramID:       programID,
			UserID:          userID,
			TicketCount:     1,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-verify-commit",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)

	resp, err := logicpkg.NewVerifyAttemptDueLogic(ctx, svcCtx).VerifyAttemptDue(&pb.VerifyAttemptDueReq{
		OrderNumber: orderNumber,
		DueAtUnix:   now.Add(-time.Second).Unix(),
	})
	if err != nil {
		t.Fatalf("VerifyAttemptDue() error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected verify attempt due success")
	}

	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateCommitted || record.ReasonCode != rush.AttemptReasonOrderCommitted {
		t.Fatalf("expected committed attempt after DB probe, got %+v", record)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 3 {
		t.Fatalf("expected committed attempt to keep quota consumed, got ok=%t available=%d", ok, available)
	}
}

func TestVerifyAttemptDueDoesNotRevertFailedPollWhenOrderArrivesLate(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	store := rebindOrderTestAttemptStore(t, svcCtx)
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Now()
	userID, programID, ticketCategoryID, viewerIDs, _ := nextRushTestIDs()
	showTimeID := programID + 301
	viewerIDs = viewerIDs[:1]
	orderNumber := sharding.BuildOrderNumber(userID, now, 1, 3)

	if err := store.SetQuotaAvailable(ctx, showTimeID, ticketCategoryID, 4); err != nil {
		t.Fatalf("SetQuotaAvailable() error = %v", err)
	}
	if _, err := store.Admit(ctx, rush.AdmitAttemptRequest{
		OrderNumber:      orderNumber,
		UserID:           userID,
		ProgramID:        programID,
		ShowTimeID:       showTimeID,
		TicketCategoryID: ticketCategoryID,
		ViewerIDs:        viewerIDs,
		TicketCount:      1,
		TokenFingerprint: rush.BuildTokenFingerprint(userID, showTimeID, ticketCategoryID, viewerIDs, "express", "paper"),
		CommitCutoffAt:   now.Add(-time.Second),
		UserDeadlineAt:   now.Add(-2 * time.Second),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	if err := store.MarkQueued(ctx, orderNumber, now); err != nil {
		t.Fatalf("MarkQueued() error = %v", err)
	}

	firstResp, err := logicpkg.NewVerifyAttemptDueLogic(ctx, svcCtx).VerifyAttemptDue(&pb.VerifyAttemptDueReq{
		OrderNumber: orderNumber,
		DueAtUnix:   now.Add(-time.Second).Unix(),
	})
	if err != nil {
		t.Fatalf("first VerifyAttemptDue() error = %v", err)
	}
	if !firstResp.GetSuccess() {
		t.Fatalf("expected first verify success")
	}

	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if record.State != rush.AttemptStateReleased || record.ReasonCode != rush.AttemptReasonCommitCutoffExceed {
		t.Fatalf("expected released attempt after cutoff verify, got %+v", record)
	}

	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              9301,
			OrderNumber:     orderNumber,
			ProgramID:       programID,
			ShowTimeID:      showTimeID,
			UserID:          userID,
			TicketCount:     1,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-verify-late-success",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)

	secondResp, err := logicpkg.NewVerifyAttemptDueLogic(ctx, svcCtx).VerifyAttemptDue(&pb.VerifyAttemptDueReq{
		OrderNumber: orderNumber,
		DueAtUnix:   now.Add(-time.Second).Unix(),
	})
	if err != nil {
		t.Fatalf("second VerifyAttemptDue() error = %v", err)
	}
	if !secondResp.GetSuccess() {
		t.Fatalf("expected second verify success")
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() after late success error = %v", err)
	}
	if record.State != rush.AttemptStateReleased {
		t.Fatalf("expected released attempt to stay failed after late order success, got %+v", record)
	}

	order, err := svcCtx.OrderRepository.FindOrderByNumber(ctx, orderNumber)
	if err != nil {
		t.Fatalf("FindOrderByNumber() error = %v", err)
	}
	if order.OrderStatus != testOrderStatusCancelled {
		t.Fatalf("expected late success order to be auto-closed, got %+v", order)
	}
}
