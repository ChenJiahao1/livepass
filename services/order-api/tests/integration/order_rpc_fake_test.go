package integration_test

import (
	"context"

	"damai-go/services/order-rpc/orderrpc"

	"google.golang.org/grpc"
)

type fakeOrderRPC struct {
	createOrderResp    *orderrpc.CreateOrderResp
	createOrderErr     error
	lastCreateOrderReq *orderrpc.CreateOrderReq

	listOrdersResp    *orderrpc.ListOrdersResp
	listOrdersErr     error
	lastListOrdersReq *orderrpc.ListOrdersReq

	getOrderResp    *orderrpc.OrderDetailInfo
	getOrderErr     error
	lastGetOrderReq *orderrpc.GetOrderReq

	getOrderServiceViewResp    *orderrpc.OrderServiceViewResp
	getOrderServiceViewErr     error
	lastGetOrderServiceViewReq *orderrpc.GetOrderServiceViewReq

	cancelOrderResp    *orderrpc.BoolResp
	cancelOrderErr     error
	lastCancelOrderReq *orderrpc.CancelOrderReq

	payOrderResp    *orderrpc.PayOrderResp
	payOrderErr     error
	lastPayOrderReq *orderrpc.PayOrderReq

	payCheckResp    *orderrpc.PayCheckResp
	payCheckErr     error
	lastPayCheckReq *orderrpc.PayCheckReq

	previewRefundOrderResp    *orderrpc.PreviewRefundOrderResp
	previewRefundOrderErr     error
	lastPreviewRefundOrderReq *orderrpc.PreviewRefundOrderReq

	refundOrderResp    *orderrpc.RefundOrderResp
	refundOrderErr     error
	lastRefundOrderReq *orderrpc.RefundOrderReq
}

func (f *fakeOrderRPC) CreateOrder(ctx context.Context, in *orderrpc.CreateOrderReq, opts ...grpc.CallOption) (*orderrpc.CreateOrderResp, error) {
	f.lastCreateOrderReq = in
	return f.createOrderResp, f.createOrderErr
}

func (f *fakeOrderRPC) ListOrders(ctx context.Context, in *orderrpc.ListOrdersReq, opts ...grpc.CallOption) (*orderrpc.ListOrdersResp, error) {
	f.lastListOrdersReq = in
	return f.listOrdersResp, f.listOrdersErr
}

func (f *fakeOrderRPC) GetOrder(ctx context.Context, in *orderrpc.GetOrderReq, opts ...grpc.CallOption) (*orderrpc.OrderDetailInfo, error) {
	f.lastGetOrderReq = in
	return f.getOrderResp, f.getOrderErr
}

func (f *fakeOrderRPC) GetOrderServiceView(ctx context.Context, in *orderrpc.GetOrderServiceViewReq, opts ...grpc.CallOption) (*orderrpc.OrderServiceViewResp, error) {
	f.lastGetOrderServiceViewReq = in
	return f.getOrderServiceViewResp, f.getOrderServiceViewErr
}

func (f *fakeOrderRPC) CancelOrder(ctx context.Context, in *orderrpc.CancelOrderReq, opts ...grpc.CallOption) (*orderrpc.BoolResp, error) {
	f.lastCancelOrderReq = in
	return f.cancelOrderResp, f.cancelOrderErr
}

func (f *fakeOrderRPC) PayOrder(ctx context.Context, in *orderrpc.PayOrderReq, opts ...grpc.CallOption) (*orderrpc.PayOrderResp, error) {
	f.lastPayOrderReq = in
	return f.payOrderResp, f.payOrderErr
}

func (f *fakeOrderRPC) PayCheck(ctx context.Context, in *orderrpc.PayCheckReq, opts ...grpc.CallOption) (*orderrpc.PayCheckResp, error) {
	f.lastPayCheckReq = in
	return f.payCheckResp, f.payCheckErr
}

func (f *fakeOrderRPC) PreviewRefundOrder(ctx context.Context, in *orderrpc.PreviewRefundOrderReq, opts ...grpc.CallOption) (*orderrpc.PreviewRefundOrderResp, error) {
	f.lastPreviewRefundOrderReq = in
	return f.previewRefundOrderResp, f.previewRefundOrderErr
}

func (f *fakeOrderRPC) RefundOrder(ctx context.Context, in *orderrpc.RefundOrderReq, opts ...grpc.CallOption) (*orderrpc.RefundOrderResp, error) {
	f.lastRefundOrderReq = in
	return f.refundOrderResp, f.refundOrderErr
}

func (f *fakeOrderRPC) CloseExpiredOrders(ctx context.Context, in *orderrpc.CloseExpiredOrdersReq, opts ...grpc.CallOption) (*orderrpc.CloseExpiredOrdersResp, error) {
	return nil, nil
}

func (f *fakeOrderRPC) CountActiveTicketsByUserProgram(ctx context.Context, in *orderrpc.CountActiveTicketsByUserProgramReq, opts ...grpc.CallOption) (*orderrpc.CountActiveTicketsByUserProgramResp, error) {
	return nil, nil
}

var _ orderrpc.OrderRpc = (*fakeOrderRPC)(nil)
