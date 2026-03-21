package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/pay-rpc/internal/logic"
	"damai-go/services/pay-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetPayBillReturnsBillByOrderNumber(t *testing.T) {
	svcCtx := newPayTestServiceContext(t)
	resetPayDomainState(t)
	seedPayBillFixtures(t, svcCtx, payBillFixture{
		OrderNumber: 91003,
		UserID:      3001,
		Subject:     "已支付订单",
		OrderAmount: 699,
	})

	l := logicpkg.NewGetPayBillLogic(context.Background(), svcCtx)
	resp, err := l.GetPayBill(&pb.GetPayBillReq{OrderNumber: 91003})
	if err != nil {
		t.Fatalf("GetPayBill returned error: %v", err)
	}
	if resp.OrderNumber != 91003 || resp.PayBillNo <= 0 || resp.PayStatus != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.PayTime != "2026-01-01 10:00:00" {
		t.Fatalf("expected local pay time, got %+v", resp)
	}
}

func TestGetPayBillReturnsNotFoundWhenBillMissing(t *testing.T) {
	svcCtx := newPayTestServiceContext(t)
	resetPayDomainState(t)

	l := logicpkg.NewGetPayBillLogic(context.Background(), svcCtx)
	_, err := l.GetPayBill(&pb.GetPayBillReq{OrderNumber: 99999})
	if err == nil {
		t.Fatalf("expected not found error")
	}
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected not found, got %s", status.Code(err))
	}
}
