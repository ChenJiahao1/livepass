package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/sharding"
	programrpc "damai-go/services/program-rpc/programrpc"

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

func TestCancelOrderWrapsReleaseBetweenTwoRepositoryTransactions(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	traceRepo := newTracingOrderRepository(svcCtx.OrderRepository)
	svcCtx.OrderRepository = traceRepo
	programRPC.onReleaseSeatFreeze = func(*programrpc.ReleaseSeatFreezeReq) {
		traceRepo.record("program:release")
	}
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:          8009,
		OrderNumber: 91009,
		ProgramID:   10001,
		UserID:      3001,
		OrderStatus: testOrderStatusUnpaid,
		FreezeToken: "freeze-cancel-split-tx",
	})
	seedOrderTicketUserFixtures(t, svcCtx, orderTicketUserFixture{
		ID:           8809,
		OrderNumber:  91009,
		UserID:       3001,
		TicketUserID: 701,
		SeatID:       501,
		SeatRow:      1,
		SeatCol:      1,
	})

	_, err := logicpkg.NewCancelOrderLogic(context.Background(), svcCtx).CancelOrder(&pb.CancelOrderReq{
		UserId:      3001,
		OrderNumber: 91009,
	})
	if err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	if traceRepo.transactByOrderNumberCalls != 2 {
		t.Fatalf("expected 2 repository transactions, got %d", traceRepo.transactByOrderNumberCalls)
	}
	expected := []string{"tx1:find", "program:release", "tx2:find", "tx2:update_cancel"}
	if got := traceRepo.events; len(got) != len(expected) {
		t.Fatalf("unexpected event count: got=%v want=%v", got, expected)
	} else {
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("unexpected event order: got=%v want=%v", got, expected)
			}
		}
	}
}

func TestCancelOrderRetriesAfterFinalizeFailure(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	resetOrderDomainState(t)
	traceRepo := newTracingOrderRepository(svcCtx.OrderRepository)
	traceRepo.failUpdateCancelStatusErr = errors.New("inject update cancel failure")
	traceRepo.failUpdateCancelStatusN = 1
	svcCtx.OrderRepository = traceRepo
	programRPC.onReleaseSeatFreeze = func(*programrpc.ReleaseSeatFreezeReq) {
		traceRepo.record("program:release")
	}
	seedOrderFixtures(t, svcCtx, orderFixture{
		ID:          8010,
		OrderNumber: 91010,
		ProgramID:   10001,
		UserID:      3001,
		OrderStatus: testOrderStatusUnpaid,
		FreezeToken: "freeze-cancel-retry",
	})
	seedOrderTicketUserFixtures(t, svcCtx, orderTicketUserFixture{
		ID:           8810,
		OrderNumber:  91010,
		UserID:       3001,
		TicketUserID: 701,
		SeatID:       501,
		SeatRow:      1,
		SeatCol:      1,
	})

	_, err := logicpkg.NewCancelOrderLogic(context.Background(), svcCtx).CancelOrder(&pb.CancelOrderReq{
		UserId:      3001,
		OrderNumber: 91010,
	})
	if err == nil {
		t.Fatalf("expected first CancelOrder to fail")
	}
	if findOrderStatus(t, testOrderMySQLDataSource, 91010) != testOrderStatusUnpaid {
		t.Fatalf("expected order to remain unpaid after finalize failure")
	}

	resp, err := logicpkg.NewCancelOrderLogic(context.Background(), svcCtx).CancelOrder(&pb.CancelOrderReq{
		UserId:      3001,
		OrderNumber: 91010,
	})
	if err != nil {
		t.Fatalf("second CancelOrder returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected retry cancel success response")
	}
	if programRPC.releaseSeatFreezeCalls != 2 {
		t.Fatalf("expected release seat retried once, got %d", programRPC.releaseSeatFreezeCalls)
	}
	if traceRepo.transactByOrderNumberCalls != 4 {
		t.Fatalf("expected 4 repository transactions, got %d", traceRepo.transactByOrderNumberCalls)
	}
	expected := []string{
		"tx1:find", "program:release", "tx2:find", "tx2:update_cancel_fail",
		"tx3:find", "program:release", "tx4:find", "tx4:update_cancel",
	}
	if got := traceRepo.events; len(got) != len(expected) {
		t.Fatalf("unexpected event count: got=%v want=%v", got, expected)
	} else {
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("unexpected event order: got=%v want=%v", got, expected)
			}
		}
	}
	if findOrderStatus(t, testOrderMySQLDataSource, 91010) != testOrderStatusCancelled {
		t.Fatalf("expected order status cancelled after retry")
	}
}

func TestCancelOrderUpdatesShardUserOrderIndexStatus(t *testing.T) {
	svcCtx, programRPC, _, _ := newOrderTestServiceContext(t)
	repeatGuard := &fakeOrderRepeatGuard{}
	svcCtx.RepeatGuard = repeatGuard
	resetOrderDomainState(t)
	setOrderTestRepositoryMode(t, svcCtx, sharding.MigrationModeShardOnly)

	userID := int64(3001)
	orderNumber := sharding.BuildOrderNumber(userID, time.Date(2026, time.March, 24, 10, 0, 0, 0, time.UTC), 1, 2)
	route := orderRouteForUser(t, svcCtx, userID)
	seedShardOrderFixtures(t, svcCtx, route, orderFixture{
		ID:          8002,
		OrderNumber: orderNumber,
		ProgramID:   10001,
		UserID:      userID,
		OrderStatus: testOrderStatusUnpaid,
		FreezeToken: "freeze-cancel-shard",
	})
	seedShardOrderTicketUserFixtures(t, svcCtx, route, orderTicketUserFixture{
		ID:           8802,
		OrderNumber:  orderNumber,
		UserID:       userID,
		TicketUserID: 702,
		SeatID:       502,
		SeatRow:      1,
		SeatCol:      2,
	})
	seedUserOrderIndexFixtures(t, svcCtx, route,
		userOrderIndexFixture{ID: 8102, OrderNumber: orderNumber, UserID: userID, ProgramID: 10001, OrderStatus: testOrderStatusUnpaid},
	)

	l := logicpkg.NewCancelOrderLogic(context.Background(), svcCtx)
	resp, err := l.CancelOrder(&pb.CancelOrderReq{
		UserId:      userID,
		OrderNumber: orderNumber,
	})
	if err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success response")
	}
	if findUserOrderIndexStatusFromTable(t, svcCtx.Config.MySQL.DataSource, "d_user_order_index_"+route.TableSuffix, orderNumber) != testOrderStatusCancelled {
		t.Fatalf("expected shard user order index status cancelled")
	}

	listResp, err := logicpkg.NewListOrdersLogic(context.Background(), svcCtx).ListOrders(&pb.ListOrdersReq{
		UserId:      userID,
		PageNumber:  1,
		PageSize:    10,
		OrderStatus: testOrderStatusCancelled,
	})
	if err != nil {
		t.Fatalf("ListOrders returned error: %v", err)
	}
	if listResp.TotalSize != 1 || len(listResp.List) != 1 || listResp.List[0].OrderNumber != orderNumber {
		t.Fatalf("expected cancelled shard list to return current order, got %+v", listResp)
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
