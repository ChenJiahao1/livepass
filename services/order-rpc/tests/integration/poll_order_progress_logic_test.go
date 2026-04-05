package integration_test

import (
	"context"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/internal/rush"
	"damai-go/services/order-rpc/pb"
)

func TestPollReturnsVerifyingAfterDeadlineWithoutTouchingDB(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	store := svcCtx.AttemptStore
	if store == nil {
		t.Fatalf("expected attempt store to be configured")
	}

	ctx := context.Background()
	now := time.Now()
	userID, programID, ticketCategoryID, viewerIDs, orderNumbers := nextRushTestIDs()
	viewerIDs = viewerIDs[:1]
	orderNumber := orderNumbers[0]

	if err := store.SetQuotaAvailable(ctx, programID, ticketCategoryID, 2); err != nil {
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
		CommitCutoffAt:   now.Add(10 * time.Millisecond),
		UserDeadlineAt:   now.Add(20 * time.Millisecond),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}

	svcCtx.OrderRepository = nil
	time.Sleep(40 * time.Millisecond)

	resp, err := logicpkg.NewPollOrderProgressLogic(ctx, svcCtx).PollOrderProgress(&pb.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("PollOrderProgress() error = %v", err)
	}
	if resp.GetOrderNumber() != orderNumber || resp.GetOrderStatus() != rush.PollOrderStatusVerifying || resp.GetDone() {
		t.Fatalf("unexpected poll response: %+v", resp)
	}
}
