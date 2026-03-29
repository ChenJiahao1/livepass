package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
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
