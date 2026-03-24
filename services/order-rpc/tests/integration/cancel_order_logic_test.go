package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCancelOrderCancelsOwnerUnpaidOrder(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	repeatGuard := &fakeOrderRepeatGuard{}
	svcCtx.RepeatGuard = repeatGuard
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

	l := logicpkg.NewCancelOrderLogic(context.Background(), svcCtx)
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
	if repeatGuard.lockCalls != 1 || repeatGuard.lastKey != "order_status:91001" {
		t.Fatalf("expected order status guard lock, got calls=%d key=%q", repeatGuard.lockCalls, repeatGuard.lastKey)
	}
	if repeatGuard.unlockCalls != 1 {
		t.Fatalf("expected order status guard unlock once, got %d", repeatGuard.unlockCalls)
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

	l := logicpkg.NewCancelOrderLogic(context.Background(), svcCtx)
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

func TestCancelOrderReturnsNotFoundForAnotherUser(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:          8001,
		OrderNumber: 91002,
		ProgramID:   10001,
		UserID:      3002,
		OrderStatus: testOrderStatusUnpaid,
		FreezeToken: "freeze-cancel-owner",
	})

	l := logicpkg.NewCancelOrderLogic(context.Background(), svcCtx)
	_, err := l.CancelOrder(&pb.CancelOrderReq{
		UserId:      3001,
		OrderNumber: 91002,
	})
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected not found, got %s", status.Code(err))
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected no release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}

func TestCancelOrderRejectsPaidOrder(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:           8001,
		OrderNumber:  91003,
		ProgramID:    10001,
		UserID:       3001,
		OrderStatus:  testOrderStatusPaid,
		FreezeToken:  "freeze-cancel-paid",
		PayOrderTime: "2026-01-02 00:00:00",
	})

	l := logicpkg.NewCancelOrderLogic(context.Background(), svcCtx)
	_, err := l.CancelOrder(&pb.CancelOrderReq{
		UserId:      3001,
		OrderNumber: 91003,
	})
	if err == nil {
		t.Fatalf("expected failed precondition error")
	}
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected failed precondition, got %s", status.Code(err))
	}
	if programRPC.releaseSeatFreezeCalls != 0 {
		t.Fatalf("expected no release call, got %d", programRPC.releaseSeatFreezeCalls)
	}
}
