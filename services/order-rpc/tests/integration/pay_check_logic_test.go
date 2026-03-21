package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	payrpc "damai-go/services/pay-rpc/payrpc"
)

func TestPayCheck(t *testing.T) {
	t.Run("returns paid bill for paid order", func(t *testing.T) {
		svcCtx, _, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89501,
			OrderNumber:     92501,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-pay-check",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   93501,
			OrderNumber: 92501,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		}

		l := logicpkg.NewPayCheckLogic(context.Background(), svcCtx)
		resp, err := l.PayCheck(&pb.PayCheckReq{
			UserId:      3001,
			OrderNumber: 92501,
		})
		if err != nil {
			t.Fatalf("PayCheck returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusPaid || resp.PayBillNo != 93501 || resp.PayStatus != 2 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("returns refunded pay status for refunded order", func(t *testing.T) {
		svcCtx, _, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89503,
			OrderNumber:     92503,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusRefunded,
			FreezeToken:     "freeze-pay-check-refunded",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   93503,
			OrderNumber: 92503,
			PayStatus:   3,
			PayTime:     "2026-12-31 19:10:00",
		}

		l := logicpkg.NewPayCheckLogic(context.Background(), svcCtx)
		resp, err := l.PayCheck(&pb.PayCheckReq{
			UserId:      3001,
			OrderNumber: 92503,
		})
		if err != nil {
			t.Fatalf("PayCheck returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusRefunded || resp.PayBillNo != 93503 || resp.PayStatus != 3 {
			t.Fatalf("unexpected refunded response: %+v", resp)
		}
	})

	t.Run("cancelled order with paid bill refunds on pay check and converges to refunded", func(t *testing.T) {
		svcCtx, _, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89504,
			OrderNumber:     92504,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusCancelled,
			FreezeToken:     "freeze-pay-check-compensation",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			CancelOrderTime: "2026-12-31 19:06:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 89641, OrderNumber: 92504, UserID: 3001, TicketUserID: 701, SeatID: 50401, OrderStatus: testOrderStatusCancelled},
			orderTicketUserFixture{ID: 89642, OrderNumber: 92504, UserID: 3001, TicketUserID: 702, SeatID: 50402, OrderStatus: testOrderStatusCancelled},
		)
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   93504,
			OrderNumber: 92504,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		}
		payRPC.refundResp = &payrpc.RefundResp{
			RefundBillNo: 94504,
			OrderNumber:  92504,
			RefundAmount: 299,
			PayStatus:    3,
			RefundTime:   "2026-12-31 19:10:00",
		}

		l := logicpkg.NewPayCheckLogic(context.Background(), svcCtx)
		resp, err := l.PayCheck(&pb.PayCheckReq{
			UserId:      3001,
			OrderNumber: 92504,
		})
		if err != nil {
			t.Fatalf("PayCheck returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusRefunded || resp.PayBillNo != 93504 || resp.PayStatus != 3 {
			t.Fatalf("unexpected compensation response: %+v", resp)
		}
		if findOrderStatus(t, testOrderMySQLDataSource, 92504) != testOrderStatusRefunded {
			t.Fatalf("expected order status refunded after compensation")
		}
		if findOrderTicketStatus(t, testOrderMySQLDataSource, 92504) != testOrderStatusRefunded {
			t.Fatalf("expected order ticket status refunded after compensation")
		}
		if payRPC.refundCalls != 1 {
			t.Fatalf("expected one refund rpc call, got %d", payRPC.refundCalls)
		}
		if payRPC.lastRefundReq == nil || payRPC.lastRefundReq.OrderNumber != 92504 || payRPC.lastRefundReq.Amount != 299 {
			t.Fatalf("unexpected refund request: %+v", payRPC.lastRefundReq)
		}
	})

	t.Run("returns unpaid status when pay bill not created", func(t *testing.T) {
		svcCtx, _, _, _ := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              89502,
			OrderNumber:     92502,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusUnpaid,
			FreezeToken:     "freeze-pay-check-unpaid",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
		})

		l := logicpkg.NewPayCheckLogic(context.Background(), svcCtx)
		resp, err := l.PayCheck(&pb.PayCheckReq{
			UserId:      3001,
			OrderNumber: 92502,
		})
		if err != nil {
			t.Fatalf("PayCheck returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusUnpaid || resp.PayBillNo != 0 || resp.PayStatus != 0 {
			t.Fatalf("unexpected unpaid response: %+v", resp)
		}
	})
}
