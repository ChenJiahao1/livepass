package integration_test

import (
	"context"
	"testing"

	logicpkg "livepass/services/pay-rpc/internal/logic"
	"livepass/services/pay-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRefund(t *testing.T) {
	t.Run("paid bill becomes refunded and inserts refund bill", func(t *testing.T) {
		svcCtx := newPayTestServiceContext(t)
		resetPayDomainState(t)
		seedPayBillFixtures(t, svcCtx, payBillFixture{
			OrderNumber: 92001,
			UserID:      3001,
			Subject:     "测试退款订单",
			OrderAmount: 499,
			PayStatus:   2,
			PayTime:     "2026-01-01 10:00:00",
		})

		l := logicpkg.NewRefundLogic(context.Background(), svcCtx)
		resp, err := l.Refund(&pb.RefundReq{
			OrderNumber: 92001,
			UserId:      3001,
			Amount:      499,
			Channel:     "mock",
			Reason:      "行程变更",
		})
		if err != nil {
			t.Fatalf("Refund returned error: %v", err)
		}
		if resp.RefundBillNo <= 0 || resp.PayStatus != 3 || resp.RefundTime == "" {
			t.Fatalf("unexpected refund response: %+v", resp)
		}
		if findPayStatusByOrderNumber(t, testPayMySQLDataSource, 92001) != 3 {
			t.Fatalf("expected pay bill status refunded")
		}
		if countRefundBillRows(t, testPayMySQLDataSource) != 1 {
			t.Fatalf("expected one refund bill row")
		}

		refundBill := findRefundBillByOrderNumber(t, testPayMySQLDataSource, 92001)
		if refundBill.RefundAmount != 499 || refundBill.RefundStatus != 2 || refundBill.RefundReason != "行程变更" {
			t.Fatalf("unexpected refund bill row: %+v", refundBill)
		}
	})

	t.Run("same orderNumber is idempotent", func(t *testing.T) {
		svcCtx := newPayTestServiceContext(t)
		resetPayDomainState(t)
		seedPayBillFixtures(t, svcCtx, payBillFixture{
			OrderNumber: 92002,
			UserID:      3001,
			Subject:     "测试退款订单",
			OrderAmount: 599,
			PayStatus:   2,
			PayTime:     "2026-01-01 10:00:00",
		})

		l := logicpkg.NewRefundLogic(context.Background(), svcCtx)
		first, err := l.Refund(&pb.RefundReq{
			OrderNumber: 92002,
			UserId:      3001,
			Amount:      599,
			Channel:     "mock",
			Reason:      "行程变更",
		})
		if err != nil {
			t.Fatalf("first Refund returned error: %v", err)
		}

		second, err := l.Refund(&pb.RefundReq{
			OrderNumber: 92002,
			UserId:      3001,
			Amount:      599,
			Channel:     "mock",
			Reason:      "重复退款",
		})
		if err != nil {
			t.Fatalf("second Refund returned error: %v", err)
		}
		if first.RefundBillNo != second.RefundBillNo {
			t.Fatalf("expected same refund bill no, first=%+v second=%+v", first, second)
		}
		if countRefundBillRows(t, testPayMySQLDataSource) != 1 {
			t.Fatalf("expected only one refund bill row")
		}
		refundBill := findRefundBillByOrderNumber(t, testPayMySQLDataSource, 92002)
		if refundBill.RefundReason != "行程变更" {
			t.Fatalf("expected original refund reason kept, got %+v", refundBill)
		}
	})

	t.Run("refund rejects when pay bill is missing", func(t *testing.T) {
		svcCtx := newPayTestServiceContext(t)
		resetPayDomainState(t)

		l := logicpkg.NewRefundLogic(context.Background(), svcCtx)
		_, err := l.Refund(&pb.RefundReq{
			OrderNumber: 92003,
			UserId:      3001,
			Amount:      499,
			Channel:     "mock",
			Reason:      "行程变更",
		})
		if err == nil {
			t.Fatalf("expected not found error")
		}
		if status.Code(err) != codes.NotFound {
			t.Fatalf("expected not found, got %s", status.Code(err))
		}
	})

	t.Run("refund rejects when pay bill is created not paid", func(t *testing.T) {
		svcCtx := newPayTestServiceContext(t)
		resetPayDomainState(t)
		seedPayBillFixtures(t, svcCtx, payBillFixture{
			OrderNumber: 92004,
			UserID:      3001,
			Subject:     "未支付退款订单",
			OrderAmount: 499,
			PayStatus:   1,
			PayTime:     "",
		})

		l := logicpkg.NewRefundLogic(context.Background(), svcCtx)
		_, err := l.Refund(&pb.RefundReq{
			OrderNumber: 92004,
			UserId:      3001,
			Amount:      499,
			Channel:     "mock",
			Reason:      "行程变更",
		})
		if err == nil {
			t.Fatalf("expected failed precondition error")
		}
		if status.Code(err) != codes.FailedPrecondition {
			t.Fatalf("expected failed precondition, got %s", status.Code(err))
		}
	})

	t.Run("refund returns existing bill when orderNumber already refunded", func(t *testing.T) {
		svcCtx := newPayTestServiceContext(t)
		resetPayDomainState(t)
		seedRefundBillFixtures(t, svcCtx, refundBillFixture{
			OrderNumber:  92005,
			UserID:       3001,
			RefundAmount: 499,
			RefundReason: "首次退款",
			RefundTime:   "2026-01-01 11:00:00",
		})

		l := logicpkg.NewRefundLogic(context.Background(), svcCtx)
		resp, err := l.Refund(&pb.RefundReq{
			OrderNumber: 92005,
			UserId:      3001,
			Amount:      499,
			Channel:     "mock",
			Reason:      "重复退款",
		})
		if err != nil {
			t.Fatalf("Refund returned error: %v", err)
		}
		if resp.OrderNumber != 92005 || resp.RefundBillNo != 96005 || resp.RefundAmount != 499 || resp.PayStatus != 3 || resp.RefundTime != "2026-01-01 11:00:00" {
			t.Fatalf("unexpected existing refund response: %+v", resp)
		}
		if countRefundBillRows(t, testPayMySQLDataSource) != 1 {
			t.Fatalf("expected only one refund bill row")
		}
	})
}
