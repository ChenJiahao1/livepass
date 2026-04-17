package integration_test

import (
	"context"

	payrpc "livepass/services/pay-rpc/payrpc"

	"google.golang.org/grpc"
)

type fakePayRPC struct {
	mockPayResp    *payrpc.MockPayResp
	mockPayErr     error
	lastMockPayReq *payrpc.MockPayReq

	getPayBillResp    *payrpc.GetPayBillResp
	getPayBillErr     error
	lastGetPayBillReq *payrpc.GetPayBillReq

	refundResp    *payrpc.RefundResp
	refundErr     error
	lastRefundReq *payrpc.RefundReq
}

func (f *fakePayRPC) MockPay(ctx context.Context, in *payrpc.MockPayReq, opts ...grpc.CallOption) (*payrpc.MockPayResp, error) {
	f.lastMockPayReq = in
	return f.mockPayResp, f.mockPayErr
}

func (f *fakePayRPC) GetPayBill(ctx context.Context, in *payrpc.GetPayBillReq, opts ...grpc.CallOption) (*payrpc.GetPayBillResp, error) {
	f.lastGetPayBillReq = in
	return f.getPayBillResp, f.getPayBillErr
}

func (f *fakePayRPC) Refund(ctx context.Context, in *payrpc.RefundReq, opts ...grpc.CallOption) (*payrpc.RefundResp, error) {
	f.lastRefundReq = in
	return f.refundResp, f.refundErr
}

var _ payrpc.PayRpc = (*fakePayRPC)(nil)
