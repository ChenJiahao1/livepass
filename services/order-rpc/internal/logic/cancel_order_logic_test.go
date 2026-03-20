package logic

import (
	"context"
	"testing"

	"damai-go/services/order-rpc/pb"
)

func TestCancelOrderCancelsOwnerUnpaidOrder(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:          8001,
		OrderNumber: 91001,
		ProgramID:   10001,
		UserID:      3001,
		OrderStatus: testOrderStatusUnpaid,
		FreezeToken: "freeze-cancel-001",
	})
	seedOrderTicketUserFixtures(t, svcCtx, orderTicketUserFixture{
		ID:           8801,
		OrderNumber:  91001,
		UserID:       3001,
		TicketUserID: 701,
		SeatID:       501,
		SeatRow:      1,
		SeatCol:      1,
	})

	l := NewCancelOrderLogic(context.Background(), svcCtx)
	resp, err := l.CancelOrder(&pb.CancelOrderReq{
		UserId:      3001,
		OrderNumber: 91001,
	})
	if err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 91001) != testOrderStatusCancelled {
		t.Fatalf("expected cancelled order status")
	}
	if findOrderTicketStatus(t, svcCtx.Config.MySQL.DataSource, 91001) != testOrderStatusCancelled {
		t.Fatalf("expected cancelled order ticket status")
	}
	if programRPC.releaseSeatFreezeCalls != 1 {
		t.Fatalf("expected one freeze release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}

func TestCancelOrderIsIdempotentForAlreadyCancelledOrder(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:              8001,
		OrderNumber:     91001,
		ProgramID:       10001,
		UserID:          3001,
		OrderStatus:     testOrderStatusCancelled,
		FreezeToken:     "freeze-cancel-002",
		CancelOrderTime: "2026-01-02 00:00:00",
	})
	seedOrderTicketUserFixtures(t, svcCtx, orderTicketUserFixture{
		ID:           8801,
		OrderNumber:  91001,
		UserID:       3001,
		TicketUserID: 701,
		SeatID:       501,
		SeatRow:      1,
		SeatCol:      1,
		OrderStatus:  testOrderStatusCancelled,
	})

	l := NewCancelOrderLogic(context.Background(), svcCtx)
	resp, err := l.CancelOrder(&pb.CancelOrderReq{
		UserId:      3001,
		OrderNumber: 91001,
	})
	if err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected idempotent success response")
	}
	if findOrderStatus(t, svcCtx.Config.MySQL.DataSource, 91001) != testOrderStatusCancelled {
		t.Fatalf("expected status unchanged as cancelled")
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected no extra release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}
