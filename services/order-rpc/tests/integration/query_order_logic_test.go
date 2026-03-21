package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestListOrdersReturnsOnlyCurrentUserOrdersAndSupportsStatusFilter(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{ID: 8001, OrderNumber: 91001, ProgramID: 10001, UserID: 3001, OrderStatus: testOrderStatusUnpaid, CreateOrderTime: "2026-01-02 00:00:00"},
		orderFixture{ID: 8002, OrderNumber: 91002, ProgramID: 10001, UserID: 3001, OrderStatus: testOrderStatusCancelled, CancelOrderTime: "2026-01-03 00:00:00"},
		orderFixture{ID: 8003, OrderNumber: 91003, ProgramID: 10001, UserID: 3002, OrderStatus: testOrderStatusUnpaid},
	)

	l := logicpkg.NewListOrdersLogic(context.Background(), svcCtx)
	resp, err := l.ListOrders(&pb.ListOrdersReq{
		UserId:      3001,
		PageNumber:  1,
		PageSize:    10,
		OrderStatus: testOrderStatusUnpaid,
	})
	if err != nil {
		t.Fatalf("ListOrders returned error: %v", err)
	}
	if resp.TotalSize != 1 || len(resp.List) != 1 {
		t.Fatalf("unexpected list response: %+v", resp)
	}
	if resp.List[0].OrderNumber != 91001 {
		t.Fatalf("unexpected order number: %+v", resp.List[0])
	}
}

func TestGetOrderReturnsDetailRowsForOwner(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{ID: 8001, OrderNumber: 91001, ProgramID: 10001, UserID: 3001, TicketCount: 2})
	seedOrderTicketUserFixtures(
		t,
		svcCtx,
		orderTicketUserFixture{ID: 8801, OrderNumber: 91001, UserID: 3001, TicketUserID: 701, SeatID: 501, SeatRow: 1, SeatCol: 1},
		orderTicketUserFixture{ID: 8802, OrderNumber: 91001, UserID: 3001, TicketUserID: 702, SeatID: 502, SeatRow: 1, SeatCol: 2},
	)

	l := logicpkg.NewGetOrderLogic(context.Background(), svcCtx)
	resp, err := l.GetOrder(&pb.GetOrderReq{
		UserId:      3001,
		OrderNumber: 91001,
	})
	if err != nil {
		t.Fatalf("GetOrder returned error: %v", err)
	}
	if resp.OrderNumber != 91001 || len(resp.OrderTicketInfoVoList) != 2 {
		t.Fatalf("unexpected order detail response: %+v", resp)
	}
}

func TestGetOrderReturnsNotFoundForAnotherUser(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(t, svcCtx, orderFixture{ID: 8001, OrderNumber: 91001, ProgramID: 10001, UserID: 3002})

	l := logicpkg.NewGetOrderLogic(context.Background(), svcCtx)
	_, err := l.GetOrder(&pb.GetOrderReq{
		UserId:      3001,
		OrderNumber: 91001,
	})
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected not found, got %s", status.Code(err))
	}
}
