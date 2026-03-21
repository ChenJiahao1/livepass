package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	payrpc "damai-go/services/pay-rpc/payrpc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPayOrder(t *testing.T) {
	t.Run("pay success updates order and ticket snapshots to paid", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
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
