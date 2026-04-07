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

func TestCloseExpiredOrderClosesOnlyExpiredUnpaidOrder(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)

	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8201,
			OrderNumber:     92001,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-one-001",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedOrderTicketUserFixtures(
		t,
		svcCtx,
		orderTicketUserFixture{ID: 8921, OrderNumber: 92001, UserID: 3001, TicketUserID: 701, SeatID: 511, SeatRow: 1, SeatCol: 1},
	)

	l := logicpkg.NewCloseExpiredOrderLogic(context.Background(), svcCtx)
	resp, err := l.CloseExpiredOrder(&pb.CloseExpiredOrderReq{OrderNumber: 92001})
	if err != nil {
		t.Fatalf("CloseExpiredOrder returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected close expired order success")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 92001) != testOrderStatusCancelled {
		t.Fatalf("expected expired unpaid order closed")
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected one release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}

func TestCloseExpiredOrderReleasesCommittedRushAttemptProjection(t *testing.T) {
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
	orderNumber := sharding.BuildOrderNumber(userID, now, 1, 3)

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
		CommitCutoffAt:   now.Add(10 * time.Second),
		UserDeadlineAt:   now.Add(15 * time.Second),
		Now:              now,
	}); err != nil {
		t.Fatalf("Admit() error = %v", err)
	}
	record, err := store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() error = %v", err)
	}
	if err := store.CommitProjection(ctx, record, nil, now); err != nil {
		t.Fatalf("CommitProjection() error = %v", err)
	}

	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8302,
			OrderNumber:     orderNumber,
			ProgramID:       programID,
			UserID:          userID,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-rush-attempt",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
	)
	seedOrderTicketUserFixtures(
		t,
		svcCtx,
		orderTicketUserFixture{ID: 8922, OrderNumber: orderNumber, UserID: userID, TicketUserID: viewerIDs[0], SeatID: 512, SeatRow: 1, SeatCol: 1},
	)

	resp, err := logicpkg.NewCloseExpiredOrderLogic(ctx, svcCtx).CloseExpiredOrder(&pb.CloseExpiredOrderReq{
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("CloseExpiredOrder() error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected close expired order success")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, orderNumber) != testOrderStatusCancelled {
		t.Fatalf("expected expired unpaid order closed")
	}

	record, err = store.Get(ctx, orderNumber)
	if err != nil {
		t.Fatalf("AttemptStore.Get() after close error = %v", err)
	}
	if record.State != rush.AttemptStateReleased || record.ReasonCode != rush.AttemptReasonClosedOrderReleased {
		t.Fatalf("expected closed order to release rush attempt projection, got %+v", record)
	}

	available, ok, err := store.GetQuotaAvailable(ctx, programID, ticketCategoryID)
	if err != nil {
		t.Fatalf("GetQuotaAvailable() error = %v", err)
	}
	if !ok || available != 4 {
		t.Fatalf("expected closed committed attempt to restore quota, got ok=%t available=%d", ok, available)
	}
}
