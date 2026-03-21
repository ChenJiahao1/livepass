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
