package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/order-rpc/internal/logic"
	"damai-go/services/order-rpc/pb"
	payrpc "damai-go/services/pay-rpc/payrpc"
	programrpc "damai-go/services/program-rpc/programrpc"
)

func TestRefundOrder(t *testing.T) {
	t.Run("paid order refunds and updates local snapshots", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              99001,
			OrderNumber:     93001,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-refund-success",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 99101, OrderNumber: 93001, UserID: 3001, TicketUserID: 701, SeatID: 50101, OrderStatus: testOrderStatusPaid},
			orderTicketUserFixture{ID: 99102, OrderNumber: 93001, UserID: 3001, TicketUserID: 702, SeatID: 50102, OrderStatus: testOrderStatusPaid},
		)
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   94001,
			OrderNumber: 93001,
			UserId:      3001,
			Amount:      598,
			PayStatus:   2,
			PayTime:     "2026-12-31 19:05:00",
		}
		payRPC.refundResp = &payrpc.RefundResp{
			RefundBillNo: 95001,
			OrderNumber:  93001,
			RefundAmount: 478,
			PayStatus:    3,
			RefundTime:   "2026-12-31 19:10:00",
		}
		programRPC.evaluateRefundRuleResp = &programrpc.EvaluateRefundRuleResp{
			AllowRefund:   true,
			RefundPercent: 80,
			RefundAmount:  478,
		}

		l := logicpkg.NewRefundOrderLogic(context.Background(), svcCtx)
		resp, err := l.RefundOrder(&pb.RefundOrderReq{
			UserId:      3001,
			OrderNumber: 93001,
			Reason:      "行程变更",
		})
		if err != nil {
			t.Fatalf("RefundOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusRefunded || resp.RefundBillNo != 95001 || resp.RefundAmount != 478 || resp.RefundPercent != 80 {
			t.Fatalf("unexpected refund response: %+v", resp)
		}
		if findOrderStatus(t, testOrderMySQLDataSource, 93001) != testOrderStatusRefunded {
			t.Fatalf("expected order status refunded")
		}
		if findOrderTicketStatus(t, testOrderMySQLDataSource, 93001) != testOrderStatusRefunded {
			t.Fatalf("expected order ticket status refunded")
		}
		if programRPC.lastEvaluateRefundRuleReq == nil || programRPC.lastEvaluateRefundRuleReq.ProgramId != 10001 {
			t.Fatalf("unexpected evaluate refund request: %+v", programRPC.lastEvaluateRefundRuleReq)
		}
		if programRPC.lastReleaseSoldSeatsReq == nil || len(programRPC.lastReleaseSoldSeatsReq.SeatIds) != 2 || programRPC.lastReleaseSoldSeatsReq.RequestNo != "refund-93001" {
			t.Fatalf("unexpected release sold seats request: %+v", programRPC.lastReleaseSoldSeatsReq)
		}
		if payRPC.lastRefundReq == nil || payRPC.lastRefundReq.OrderNumber != 93001 || payRPC.lastRefundReq.Amount != 478 {
			t.Fatalf("unexpected refund rpc request: %+v", payRPC.lastRefundReq)
		}
	})

	t.Run("paid order converges when pay bill already refunded", func(t *testing.T) {
		svcCtx, programRPC, _, payRPC := newOrderTestServiceContext(t)
		resetOrderDomainState(t)
		seedOrderFixtures(t, svcCtx, orderFixture{
			ID:              99002,
			OrderNumber:     93002,
			ProgramID:       10001,
			UserID:          3001,
			OrderStatus:     testOrderStatusPaid,
			FreezeToken:     "freeze-refund-converge",
			OrderExpireTime: "2026-12-31 20:00:00",
			CreateOrderTime: "2026-12-31 19:00:00",
			PayOrderTime:    "2026-12-31 19:05:00",
		})
		seedOrderTicketUserFixtures(t, svcCtx,
			orderTicketUserFixture{ID: 99201, OrderNumber: 93002, UserID: 3001, TicketUserID: 701, SeatID: 50201, OrderStatus: testOrderStatusPaid},
			orderTicketUserFixture{ID: 99202, OrderNumber: 93002, UserID: 3001, TicketUserID: 702, SeatID: 50202, OrderStatus: testOrderStatusPaid},
		)
		payRPC.getPayBillResp = &payrpc.GetPayBillResp{
			PayBillNo:   94002,
			OrderNumber: 93002,
			UserId:      3001,
			Amount:      598,
			PayStatus:   3,
			PayTime:     "2026-12-31 19:05:00",
		}
		payRPC.refundResp = &payrpc.RefundResp{
			RefundBillNo: 95002,
			OrderNumber:  93002,
			RefundAmount: 478,
			PayStatus:    3,
			RefundTime:   "2026-12-31 19:12:00",
		}

		l := logicpkg.NewRefundOrderLogic(context.Background(), svcCtx)
		resp, err := l.RefundOrder(&pb.RefundOrderReq{
			UserId:      3001,
			OrderNumber: 93002,
			Reason:      "重复发起退款",
		})
		if err != nil {
			t.Fatalf("RefundOrder returned error: %v", err)
		}
		if resp.OrderStatus != testOrderStatusRefunded || resp.RefundBillNo != 95002 || resp.RefundAmount != 478 {
			t.Fatalf("unexpected convergence response: %+v", resp)
		}
		if findOrderStatus(t, testOrderMySQLDataSource, 93002) != testOrderStatusRefunded {
			t.Fatalf("expected order status refunded after convergence")
		}
		if programRPC.lastEvaluateRefundRuleReq != nil {
			t.Fatalf("expected no rule evaluation when pay bill already refunded, got %+v", programRPC.lastEvaluateRefundRuleReq)
		}
		if programRPC.lastReleaseSoldSeatsReq == nil || programRPC.lastReleaseSoldSeatsReq.RequestNo != "refund-93002" {
			t.Fatalf("expected release sold seats request, got %+v", programRPC.lastReleaseSoldSeatsReq)
		}
	})
}
