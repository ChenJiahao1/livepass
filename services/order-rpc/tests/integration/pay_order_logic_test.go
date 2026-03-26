package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	"damai-go/services/order-rpc/sharding"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPayOrder(t *testing.T) {
	t.Run("pay success updates shard order and ticket snapshots in shard only mode", func(t *testing.T) {
		svcCtx, _, _, payRPC := newOrderTestServiceContext(t)
		repeatGuard := &fakeOrderRepeatGuard{}
		svcCtx.RepeatGuard = repeatGuard
		resetOrderDomainState(t)

		userID := int64(3001)
		orderNumber := sharding.BuildOrderNumber(userID, time.Date(2026, time.March, 24, 10, 0, 0, 0, time.UTC), 1, 1)
		route := orderRouteForUser(t, svcCtx, userID)
		seedShardOrderFixtures(t, svcCtx, route, orderFixture{
			ID:              89001,
			OrderNumber:     orderNumber,
			ProgramID:       10001,
			UserID:          userID,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-pay-shard",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
		})
		seedShardOrderTicketUserFixtures(t, svcCtx, route,
			orderTicketUserFixture{ID: 89101, OrderNumber: orderNumber, UserID: userID, TicketUserID: 701, OrderStatus: testOrderStatusUnpaid},
		)
		payRPC.mockPayResp = &payrpc.MockPayResp{
			PayBillNo: 93001,
			PayStatus: 2,
			PayTime:   "2026-12-31 19:05:00",
		}

		l := logicpkg.NewPayOrderLogic(context.Background(), svcCtx)
		resp, err := l.PayOrder(&pb.PayOrderReq{
			UserId:      userID,
			OrderNumber: orderNumber,
			Subject:     "演出票支付",
			Channel:     "mock",
		})
		if err != nil {
			t.Fatalf("PayOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusPaid {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if countShardOrderRows(t, testOrderMySQLDataSource) != 1 {
			t.Fatalf("expected exactly one shard order row after pay")
		}
		if findOrderStatusFromTable(t, testOrderMySQLDataSource, "d_order_"+route.TableSuffix, orderNumber) != testOrderStatusPaid {
			t.Fatalf("expected shard order status paid")
		}
		if findOrderTicketStatusFromTable(t, testOrderMySQLDataSource, "d_order_ticket_user_"+route.TableSuffix, orderNumber) != testOrderStatusPaid {
			t.Fatalf("expected shard order ticket status paid")
		}

		listResp, err := logicpkg.NewListOrdersLogic(context.Background(), svcCtx).ListOrders(&pb.ListOrdersReq{
			UserId:      userID,
			PageNumber:  1,
			PageSize:    10,
			OrderStatus: testOrderStatusPaid,
		})
		if err != nil {
			t.Fatalf("ListOrders returned error: %v", err)
		}
		if listResp.TotalSize != 1 || len(listResp.List) != 1 || listResp.List[0].OrderNumber != orderNumber {
			t.Fatalf("expected paid shard list to return current order, got %+v", listResp)
		}
	})

	t.Run("pay success updates order and ticket snapshots to paid", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		repeatGuard := &fakeOrderRepeatGuard{}
		svcCtx.RepeatGuard = repeatGuard
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89001,
			OrderNumber:     92001,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-pay-success",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 89101, OrderNumber: 92001, UserID: 3001, TicketUserID: 701, OrderStatus: testOrderStatusUnpaid},
			orderTicketUserFixture{ID: 89102, OrderNumber: 92001, UserID: 3001, TicketUserID: 702, OrderStatus: testOrderStatusUnpaid},
		)
		payRPC.mockPayResp = &payrpc.MockPayResp{
			PayBillNo: 93001,
			PayStatus: 2,
			PayTime:   "2026-12-31 19:05:00",
		}

		l := logicpkg.NewPayOrderLogic(context.Background(), svcCtx)
		resp, err := l.PayOrder(&pb.PayOrderReq{
			UserId:      3001,
			OrderNumber: 92001,
			Subject:     "演出票支付",
			Channel:     "mock",
		})
		if err != nil {
			t.Fatalf("PayOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusPaid || resp.PayBillNo != 93001 || resp.PayStatus != 2 {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if findOrderStatus(t, testOrderMySQLDataSource, 92001) != testOrderStatusPaid {
			t.Fatalf("expected order status paid")
		}
		if findOrderTicketStatus(t, testOrderMySQLDataSource, 92001) != testOrderStatusPaid {
			t.Fatalf("expected order ticket status paid")
		}
		if payRPC.lastMockPayReq == nil || payRPC.lastMockPayReq.OrderNumber != 92001 || payRPC.lastMockPayReq.UserId != 3001 {
			t.Fatalf("unexpected mock pay request: %+v", payRPC.lastMockPayReq)
		}
		if programRPC.lastConfirmSeatFreezeReq == nil || programRPC.lastConfirmSeatFreezeReq.FreezeToken != "freeze-pay-success" {
			t.Fatalf("unexpected confirm request: %+v", programRPC.lastConfirmSeatFreezeReq)
		}
		if repeatGuard.lockCalls != 1 || repeatGuard.lastKey != "order_status:92001" {
			t.Fatalf("expected order status guard lock, got calls=%d key=%q", repeatGuard.lockCalls, repeatGuard.lastKey)
		}
		if repeatGuard.unlockCalls != 1 {
			t.Fatalf("expected order status guard unlock once, got %d", repeatGuard.unlockCalls)
		}

		getResp, err := logicpkg.NewGetOrderLogic(context.Background(), svcCtx).GetOrder(&pb.GetOrderReq{
			UserId:      3001,
			OrderNumber: 92001,
		})
		if err != nil {
			t.Fatalf("GetOrder returned error: %v", err)
		}
		if getResp.OrderStatus != testOrderStatusPaid {
			t.Fatalf("expected get order paid, got %+v", getResp)
		}

		listResp, err := logicpkg.NewListOrdersLogic(context.Background(), svcCtx).ListOrders(&pb.ListOrdersReq{
			UserId:     3001,
			PageNumber: 1,
			PageSize:   10,
		})
		if err != nil {
			t.Fatalf("ListOrders returned error: %v", err)
		}
		if len(listResp.List) != 1 || listResp.List[0].OrderStatus != testOrderStatusPaid {
			t.Fatalf("expected list order paid, got %+v", listResp)
		}
	})

	t.Run("pay wraps downstream rpc between two repository transactions", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		traceRepo := newTracingOrderRepository(svcCtx.OrderRepository)
		svcCtx.OrderRepository = traceRepo
		payRPC.onMockPay = func(*payrpc.MockPayReq) {
			traceRepo.record("pay:mock")
		}
		programRPC.onConfirmSeatFreeze = func(*programrpc.ConfirmSeatFreezeReq) {
			traceRepo.record("program:confirm")
		}
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89011,
			OrderNumber:     92111,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-pay-split-tx",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 89111, OrderNumber: 92111, UserID: 3001, TicketUserID: 701, OrderStatus: testOrderStatusUnpaid},
		)
		payRPC.mockPayResp = &payrpc.MockPayResp{
			PayBillNo: 93111,
			PayStatus: 2,
			PayTime:   "2026-12-31 19:05:00",
		}

		_, err := logicpkg.NewPayOrderLogic(context.Background(), svcCtx).PayOrder(&pb.PayOrderReq{
			UserId:      3001,
			OrderNumber: 92111,
			Subject:     "演出票支付",
			Channel:     "mock",
		})
		if err != nil {
			t.Fatalf("PayOrder returned error: %v", err)
		}
		if traceRepo.transactByOrderNumberCalls != 2 {
			t.Fatalf("expected 2 repository transactions, got %d", traceRepo.transactByOrderNumberCalls)
		}
		expected := []string{"tx1:find", "pay:mock", "program:confirm", "tx2:find", "tx2:update_pay"}
		if got := traceRepo.events; len(got) != len(expected) {
			t.Fatalf("unexpected event count: got=%v want=%v", got, expected)
		} else {
			for i := range expected {
				if got[i] != expected[i] {
					t.Fatalf("unexpected event order: got=%v want=%v", got, expected)
				}
			}
		}
	})

	t.Run("retry after finalize pay failure eventually marks order paid", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		traceRepo := newTracingOrderRepository(svcCtx.OrderRepository)
		traceRepo.failUpdatePayStatusErr = errors.New("inject update pay failure")
		traceRepo.failUpdatePayStatusN = 1
		svcCtx.OrderRepository = traceRepo
		payRPC.onMockPay = func(*payrpc.MockPayReq) {
			traceRepo.record("pay:mock")
		}
		programRPC.onConfirmSeatFreeze = func(*programrpc.ConfirmSeatFreezeReq) {
			traceRepo.record("program:confirm")
		}
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89012,
			OrderNumber:     92112,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-pay-retry",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 89112, OrderNumber: 92112, UserID: 3001, TicketUserID: 701, OrderStatus: testOrderStatusUnpaid},
		)
		payRPC.mockPayResp = &payrpc.MockPayResp{
			PayBillNo: 93112,
			PayStatus: 2,
			PayTime:   "2026-12-31 19:05:00",
		}

		l := logicpkg.NewPayOrderLogic(context.Background(), svcCtx)
		if _, err := l.PayOrder(&pb.PayOrderReq{
			UserId:      3001,
			OrderNumber: 92112,
			Subject:     "演出票支付",
			Channel:     "mock",
		}); err == nil {
			t.Fatalf("expected first PayOrder to fail")
		}
		if findOrderStatus(t, testOrderMySQLDataSource, 92112) != testOrderStatusUnpaid {
			t.Fatalf("expected order to remain unpaid after finalize failure")
		}

		resp, err := l.PayOrder(&pb.PayOrderReq{
			UserId:      3001,
			OrderNumber: 92112,
			Subject:     "演出票支付",
			Channel:     "mock",
		})
		if err != nil {
			t.Fatalf("second PayOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusPaid || resp.PayBillNo != 93112 {
			t.Fatalf("unexpected response after retry: %+v", resp)
		}
		if payRPC.mockPayCalls != 2 {
			t.Fatalf("expected mock pay retried once, got %d", payRPC.mockPayCalls)
		}
		if programRPC.confirmSeatFreezeCalls != 2 {
			t.Fatalf("expected confirm seat retried once, got %d", programRPC.confirmSeatFreezeCalls)
		}
		if traceRepo.transactByOrderNumberCalls != 4 {
			t.Fatalf("expected 4 repository transactions, got %d", traceRepo.transactByOrderNumberCalls)
		}
		expected := []string{
			"tx1:find", "pay:mock", "program:confirm", "tx2:find", "tx2:update_pay_fail",
			"tx3:find", "pay:mock", "program:confirm", "tx4:find", "tx4:update_pay",
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
		if findOrderStatus(t, testOrderMySQLDataSource, 92112) != testOrderStatusPaid {
			t.Fatalf("expected order status paid after retry")
		}
	})

	t.Run("repeated pay on paid order returns existing paid result", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89002,
			OrderNumber:     92002,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-pay-repeat",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   93002,
			OrderNumber: 92002,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		}

		l := logicpkg.NewPayOrderLogic(context.Background(), svcCtx)
		resp, err := l.PayOrder(&pb.PayOrderReq{
			UserId:      3001,
			OrderNumber: 92002,
			Subject:     "演出票支付",
		})
		if err != nil {
			t.Fatalf("PayOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusPaid || resp.PayBillNo != 93002 {
			t.Fatalf("unexpected response: %+v", resp)
		}
		if payRPC.mockPayCalls != 0 {
			t.Fatalf("expected no mock pay call, got %d", payRPC.mockPayCalls)
		}
		if programRPC.confirmSeatFreezeCalls != 0 {
			t.Fatalf("expected no confirm seat call, got %d", programRPC.confirmSeatFreezeCalls)
		}
	})

	t.Run("expired unpaid order cannot be paid", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89003,
			OrderNumber:     92003,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-pay-expired",
			OrderExpireTime: "2026-01-01 10:00:00",
			CreateOrderTime: "2026-01-01 09:00:00",
		})

		l := logicpkg.NewPayOrderLogic(context.Background(), svcCtx)
		_, err := l.PayOrder(&pb.PayOrderReq{
			UserId:      3001,
			OrderNumber: 92003,
			Subject:     "演出票支付",
		})
		if err == nil {
			t.Fatalf("expected failed precondition error")
		}
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected failed precondition, got %s", status.Code(err))
		}
		if payRPC.mockPayCalls != 0 || programRPC.confirmSeatFreezeCalls != 0 {
			t.Fatalf("expected no downstream calls, pay=%d confirm=%d", payRPC.mockPayCalls, programRPC.confirmSeatFreezeCalls)
		}
	})

	t.Run("other user cannot pay current order", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89004,
			OrderNumber:     92004,
			ProgramID:       10001,
			UserID:          3002,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-pay-owner",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
		})

		l := logicpkg.NewPayOrderLogic(context.Background(), svcCtx)
		_, err := l.PayOrder(&pb.PayOrderReq{
			UserId:      3001,
			OrderNumber: 92004,
			Subject:     "演出票支付",
		})
		if err == nil {
			t.Fatalf("expected not found error")
		}
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected not found, got %s", status.Code(err))
		}
		if payRPC.mockPayCalls != 0 || programRPC.confirmSeatFreezeCalls != 0 {
			t.Fatalf("expected no downstream calls, pay=%d confirm=%d", payRPC.mockPayCalls, programRPC.confirmSeatFreezeCalls)
		}
	})
}
