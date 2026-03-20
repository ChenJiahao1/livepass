package logic

import (
	"context"
	"testing"

	"damai-go/services/pay-rpc/pb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMockPayCreatesPaidBill(t *testing.T) {
	svcCtx := newPayTestServiceContext(t)
	resetPayDomainState(t)

	l := NewMockPayLogic(context.Background(), svcCtx)
	resp, err := l.MockPay(&pb.MockPayReq{
		OrderNumber: 91001,
		UserId:      3001,
		Subject:     "测试订单",
		Amount:      499,
	})
	if err != nil {
		t.Fatalf("MockPay returned error: %v", err)
	}
	if resp.PayBillNo <= 0 || resp.PayStatus != 2 || resp.PayTime == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if countPayBillRows(t, testPayMySQLDataSource) != 1 {
		t.Fatalf("expected one pay bill row")
	}
}

func TestMockPayIsIdempotentByOrderNumber(t *testing.T) {
	svcCtx := newPayTestServiceContext(t)
	resetPayDomainState(t)

	l := NewMockPayLogic(context.Background(), svcCtx)
	first, err := l.MockPay(&pb.MockPayReq{
		OrderNumber: 91002,
		UserId:      3001,
		Subject:     "测试订单",
		Channel:     "mock",
		Amount:      599,
	})
	if err != nil {
		t.Fatalf("first MockPay returned error: %v", err)
	}
	second, err := l.MockPay(&pb.MockPayReq{
		OrderNumber: 91002,
		UserId:      3001,
		Subject:     "测试订单",
		Channel:     "mock",
		Amount:      599,
	})
	if err != nil {
		t.Fatalf("second MockPay returned error: %v", err)
	}
	if first.PayBillNo != second.PayBillNo {
		t.Fatalf("expected idempotent pay bill no, first=%+v second=%+v", first, second)
	}
	if countPayBillRows(t, testPayMySQLDataSource) != 1 {
		t.Fatalf("expected only one pay bill row")
	}
}

func TestMockPayRejectsInvalidParam(t *testing.T) {
	svcCtx := newPayTestServiceContext(t)
	resetPayDomainState(t)

	l := NewMockPayLogic(context.Background(), svcCtx)
	_, err := l.MockPay(&pb.MockPayReq{
		OrderNumber: 0,
		UserId:      3001,
		Subject:     "测试订单",
		Amount:      599,
	})
	if err == nil {
		t.Fatalf("expected invalid argument error")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument, got %s", status.Code(err))
	}
}
