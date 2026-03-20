package logic

import (
	"context"
	"testing"

	"damai-go/services/order-rpc/pb"
)

func TestCloseExpiredOrdersClosesOnlyExpiredUnpaidRows(t *testing.T) {
	svcCtx, programRPC, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{
			ID:              8001,
			OrderNumber:     91001,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-001",
			OrderExpireTime: "2026-01-01 00:00:00",
		},
		orderFixture{
			ID:              8002,
			OrderNumber:     91002,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-close-002",
			OrderExpireTime: "2099-01-01 00:00:00",
		},
		orderFixture{
			ID:              8003,
			OrderNumber:     91003,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusCancelled,
			FreezeToken:     "freeze-close-003",
			OrderExpireTime: "2026-01-01 00:00:00",
			CancelOrderTime: "2026-01-01 00:10:00",
		},
	)
	seedOrderTicketUserFixtures(
		t,
		svcCtx,
		orderTicketUserFixture{ID: 8801, OrderNumber: 91001, UserID: 3001, TicketUserID: 701, SeatID: 501, SeatRow: 1, SeatCol: 1},
		orderTicketUserFixture{ID: 8802, OrderNumber: 91002, UserID: 3001, TicketUserID: 702, SeatID: 502, SeatRow: 1, SeatCol: 2},
		orderTicketUserFixture{ID: 8803, OrderNumber: 91003, UserID: 3001, TicketUserID: 703, SeatID: 503, SeatRow: 1, SeatCol: 3, OrderStatus: testOrderStatusCancelled},
	)

	l := NewCloseExpiredOrdersLogic(context.Background(), svcCtx)
	resp, err := l.CloseExpiredOrders(&pb.CloseExpiredOrdersReq{Limit: 10})
	if err != nil {
		t.Fatalf("CloseExpiredOrders returned error: %v", err)
	}
	if resp.ClosedCount != 1 {
		t.Fatalf("expected one closed order, got %d", resp.ClosedCount)
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 91001) != testOrderStatusCancelled {
		t.Fatalf("expected expired unpaid order closed")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 91002) != testOrderStatusUnpaid {
		t.Fatalf("expected non-expired order unchanged")
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected one release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}
