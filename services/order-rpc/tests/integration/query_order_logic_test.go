package integration_test

import (
	"context"
	"testing"
	"time"

	"damai-go/pkg/xid"
	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/sharding"

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

func TestCountActiveTicketsByUserProgramIncludesPaidOnly(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	seedOrderFixtures(
		t,
		svcCtx,
		orderFixture{ID: 8001, OrderNumber: 91001, ProgramID: 10001, UserID: 3001, TicketCount: 2, OrderStatus: testOrderStatusUnpaid},
		orderFixture{ID: 8002, OrderNumber: 91002, ProgramID: 10001, UserID: 3001, TicketCount: 3, OrderStatus: testOrderStatusPaid, PayOrderTime: "2026-01-01 01:00:00"},
		orderFixture{ID: 8003, OrderNumber: 91003, ProgramID: 10001, UserID: 3001, TicketCount: 4, OrderStatus: testOrderStatusCancelled, CancelOrderTime: "2026-01-01 02:00:00"},
		orderFixture{ID: 8004, OrderNumber: 91004, ProgramID: 10001, UserID: 3001, TicketCount: 5, OrderStatus: testOrderStatusRefunded, PayOrderTime: "2026-01-01 03:00:00"},
		orderFixture{ID: 8005, OrderNumber: 91005, ProgramID: 10002, UserID: 3001, TicketCount: 6, OrderStatus: testOrderStatusPaid, PayOrderTime: "2026-01-01 04:00:00"},
		orderFixture{ID: 8006, OrderNumber: 91006, ProgramID: 10001, UserID: 3002, TicketCount: 7, OrderStatus: testOrderStatusPaid, PayOrderTime: "2026-01-01 05:00:00"},
	)

	l := logicpkg.NewCountActiveTicketsByUserProgramLogic(context.Background(), svcCtx)
	resp, err := l.CountActiveTicketsByUserProgram(&pb.CountActiveTicketsByUserProgramReq{
		UserId:    3001,
		ProgramId: 10001,
	})
	if err != nil {
		t.Fatalf("CountActiveTicketsByUserProgram returned error: %v", err)
	}
	if resp.ActiveTicketCount != 5 {
		t.Fatalf("expected activeTicketCount=5, got %d", resp.ActiveTicketCount)
	}
}

func TestGetOrderReadsFromShardByGeneOrderNumber(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	setOrderTestRepositoryMode(t, svcCtx, sharding.MigrationModeShardOnly)

	userID := int64(3001)
	orderNumber := sharding.BuildOrderNumber(userID, testOrderNow(), 1, 1)
	route := orderRouteForUser(t, svcCtx, userID)
	seedShardOrderFixtures(t, svcCtx, route, orderFixture{ID: 8001, OrderNumber: orderNumber, ProgramID: 10001, UserID: userID, TicketCount: 1})
	seedShardOrderTicketUserFixtures(t, svcCtx, route, orderTicketUserFixture{ID: 8801, OrderNumber: orderNumber, UserID: userID, TicketUserID: 701, SeatID: 501, SeatRow: 1, SeatCol: 1})

	l := logicpkg.NewGetOrderLogic(context.Background(), svcCtx)
	resp, err := l.GetOrder(&pb.GetOrderReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("GetOrder returned error: %v", err)
	}
	if resp.OrderNumber != orderNumber || len(resp.OrderTicketInfoVoList) != 1 {
		t.Fatalf("unexpected order detail response: %+v", resp)
	}
}

func TestGetOrderReadsLegacyOrderThroughRouteDirectoryInShardOnlyMode(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	setOrderTestRepositoryMode(t, svcCtx, sharding.MigrationModeShardOnly)

	userID := int64(3001)
	orderNumber := xid.New()
	route := orderRouteForUser(t, svcCtx, userID)
	seedShardOrderFixtures(t, svcCtx, route, orderFixture{ID: 8001, OrderNumber: orderNumber, ProgramID: 10001, UserID: userID, TicketCount: 1})
	seedShardOrderTicketUserFixtures(t, svcCtx, route, orderTicketUserFixture{ID: 8801, OrderNumber: orderNumber, UserID: userID, TicketUserID: 701, SeatID: 501, SeatRow: 1, SeatCol: 1})
	seedLegacyOrderRouteFixtures(t, svcCtx, legacyOrderRouteFixture{
		OrderNumber:  orderNumber,
		UserID:       userID,
		LogicSlot:    int64(route.LogicSlot),
		RouteVersion: route.Version,
	})

	l := logicpkg.NewGetOrderLogic(context.Background(), svcCtx)
	resp, err := l.GetOrder(&pb.GetOrderReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("GetOrder returned error: %v", err)
	}
	if resp.OrderNumber != orderNumber || len(resp.OrderTicketInfoVoList) != 1 {
		t.Fatalf("unexpected legacy order detail response: %+v", resp)
	}
}

func TestListOrdersReadsFromShardOrderTableDirectly(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	setOrderTestRepositoryMode(t, svcCtx, sharding.MigrationModeShardOnly)

	userID := int64(3001)
	orderNumber1 := sharding.BuildOrderNumber(userID, testOrderNow(), 1, 1)
	orderNumber2 := sharding.BuildOrderNumber(userID, testOrderNow().Add(time.Second), 1, 2)
	route := orderRouteForUser(t, svcCtx, userID)
	seedShardOrderFixtures(t, svcCtx, route,
		orderFixture{ID: 8001, OrderNumber: orderNumber1, ProgramID: 10001, UserID: userID, OrderStatus: testOrderStatusUnpaid, CreateOrderTime: "2026-03-24 10:00:00"},
		orderFixture{ID: 8002, OrderNumber: orderNumber2, ProgramID: 10001, UserID: userID, OrderStatus: testOrderStatusCancelled, CancelOrderTime: "2026-03-24 10:01:00", CreateOrderTime: "2026-03-24 10:01:00"},
	)

	l := logicpkg.NewListOrdersLogic(context.Background(), svcCtx)
	resp, err := l.ListOrders(&pb.ListOrdersReq{
		UserId:     userID,
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatalf("ListOrders returned error: %v", err)
	}
	if resp.TotalSize != 2 || len(resp.List) != 2 {
		t.Fatalf("unexpected list response: %+v", resp)
	}
}

func testOrderNow() time.Time {
	return time.Date(2026, time.March, 24, 10, 0, 0, 0, time.UTC)
}
