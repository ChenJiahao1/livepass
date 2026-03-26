package integration_test

import (
	"context"
	"testing"

	logicpkg "damai-go/services/pay-api/internal/logic"
	"damai-go/services/pay-api/internal/svc"
	"damai-go/services/pay-api/internal/types"
	payrpc "damai-go/services/pay-rpc/payrpc"
)

func newPayAPIServiceContext(fakeRPC *fakePayRPC) *svc.ServiceContext {
	return &svc.ServiceContext{
		PayRpc: fakeRPC,
	}
}

func TestCommonPayForwardsMockPayRequest(t *testing.T) {
	fakeRPC := &fakePayRPC{
		mockPayResp: &payrpc.MockPayResp{
			PayBillNo: 92001,
			PayStatus: 2,
			PayTime:   "2026-03-26 10:00:00",
		},
	}

	l := logicpkg.NewCommonPayLogic(context.Background(), newPayAPIServiceContext(fakeRPC))
	resp, err := l.CommonPay(&types.CommonPayReq{
		OrderNumber: 91001,
		UserID:      3001,
		Subject:     "演出票支付",
		Channel:     "mock",
		Amount:      499,
	})
	if err != nil {
		t.Fatalf("CommonPay returned error: %v", err)
	}
	if resp.PayBillNo != 92001 || resp.PayStatus != 2 || resp.PayTime != "2026-03-26 10:00:00" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastMockPayReq == nil ||
		fakeRPC.lastMockPayReq.OrderNumber != 91001 ||
		fakeRPC.lastMockPayReq.UserId != 3001 ||
		fakeRPC.lastMockPayReq.Subject != "演出票支付" ||
		fakeRPC.lastMockPayReq.Channel != "mock" ||
		fakeRPC.lastMockPayReq.Amount != 499 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastMockPayReq)
	}
}

func TestGetPayBillForwardsOrderNumber(t *testing.T) {
	fakeRPC := &fakePayRPC{
		getPayBillResp: &payrpc.GetPayBillResp{
			PayBillNo:   92001,
			OrderNumber: 91001,
			UserId:      3001,
			Subject:     "演出票支付",
			Channel:     "mock",
			Amount:      499,
			PayStatus:   2,
			PayTime:     "2026-03-26 10:00:00",
		},
	}

	l := logicpkg.NewGetPayBillLogic(context.Background(), newPayAPIServiceContext(fakeRPC))
	resp, err := l.GetPayBill(&types.PayBillReq{OrderNumber: 91001})
	if err != nil {
		t.Fatalf("GetPayBill returned error: %v", err)
	}
	if resp.PayBillNo != 92001 || resp.UserID != 3001 || resp.Amount != 499 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastGetPayBillReq == nil || fakeRPC.lastGetPayBillReq.OrderNumber != 91001 {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastGetPayBillReq)
	}
}

func TestRefundLoadsBillUserAndForwardsRefundRequest(t *testing.T) {
	fakeRPC := &fakePayRPC{
		getPayBillResp: &payrpc.GetPayBillResp{
			PayBillNo:   92001,
			OrderNumber: 91001,
			UserId:      3001,
			Subject:     "演出票支付",
			Channel:     "mock",
			Amount:      499,
			PayStatus:   2,
			PayTime:     "2026-03-26 10:00:00",
		},
		refundResp: &payrpc.RefundResp{
			RefundBillNo: 93001,
			OrderNumber:  91001,
			RefundAmount: 499,
			PayStatus:    3,
			RefundTime:   "2026-03-26 10:05:00",
		},
	}

	l := logicpkg.NewRefundLogic(context.Background(), newPayAPIServiceContext(fakeRPC))
	resp, err := l.Refund(&types.RefundReq{
		OrderNumber: 91001,
		Amount:      499,
		Channel:     "mock",
		Reason:      "行程变更",
	})
	if err != nil {
		t.Fatalf("Refund returned error: %v", err)
	}
	if resp.RefundBillNo != 93001 || resp.RefundAmount != 499 || resp.PayStatus != 3 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if fakeRPC.lastGetPayBillReq == nil || fakeRPC.lastGetPayBillReq.OrderNumber != 91001 {
		t.Fatalf("expected pay bill lookup before refund, got %+v", fakeRPC.lastGetPayBillReq)
	}
	if fakeRPC.lastRefundReq == nil ||
		fakeRPC.lastRefundReq.OrderNumber != 91001 ||
		fakeRPC.lastRefundReq.UserId != 3001 ||
		fakeRPC.lastRefundReq.Amount != 499 ||
		fakeRPC.lastRefundReq.Channel != "mock" ||
		fakeRPC.lastRefundReq.Reason != "行程变更" {
		t.Fatalf("unexpected rpc request: %+v", fakeRPC.lastRefundReq)
	}
}
