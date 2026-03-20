package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"damai-go/jobs/order-close/internal/config"
	"damai-go/jobs/order-close/internal/svc"
	"damai-go/services/order-rpc/orderrpc"

	"google.golang.org/grpc"
)

type fakeJobOrderRPC struct {
	closeExpiredOrdersResp    *orderrpc.CloseExpiredOrdersResp
	closeExpiredOrdersErr     error
	lastCloseExpiredOrdersReq *orderrpc.CloseExpiredOrdersReq
}

func (f *fakeJobOrderRPC) CreateOrder(ctx context.Context, in *orderrpc.CreateOrderReq, opts ...grpc.CallOption) (*orderrpc.CreateOrderResp, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) ListOrders(ctx context.Context, in *orderrpc.ListOrdersReq, opts ...grpc.CallOption) (*orderrpc.ListOrdersResp, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) GetOrder(ctx context.Context, in *orderrpc.GetOrderReq, opts ...grpc.CallOption) (*orderrpc.OrderDetailInfo, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) CancelOrder(ctx context.Context, in *orderrpc.CancelOrderReq, opts ...grpc.CallOption) (*orderrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) PayOrder(ctx context.Context, in *orderrpc.PayOrderReq, opts ...grpc.CallOption) (*orderrpc.PayOrderResp, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) PayCheck(ctx context.Context, in *orderrpc.PayCheckReq, opts ...grpc.CallOption) (*orderrpc.PayCheckResp, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) CloseExpiredOrders(ctx context.Context, in *orderrpc.CloseExpiredOrdersReq, opts ...grpc.CallOption) (*orderrpc.CloseExpiredOrdersResp, error) {
	f.lastCloseExpiredOrdersReq = in
	return f.closeExpiredOrdersResp, f.closeExpiredOrdersErr
}

func (f *fakeJobOrderRPC) CountActiveTicketsByUserProgram(ctx context.Context, in *orderrpc.CountActiveTicketsByUserProgramReq, opts ...grpc.CallOption) (*orderrpc.CountActiveTicketsByUserProgramResp, error) {
	return nil, nil
}

func TestRunOnceForwardsBatchLimit(t *testing.T) {
	fakeRPC := &fakeJobOrderRPC{
		closeExpiredOrdersResp: &orderrpc.CloseExpiredOrdersResp{ClosedCount: 3},
	}
	l := NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config: config.Config{
			Interval:  time.Minute,
			BatchSize: 100,
		},
		OrderRpc: fakeRPC,
	})

	if err := l.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if fakeRPC.lastCloseExpiredOrdersReq == nil || fakeRPC.lastCloseExpiredOrdersReq.Limit != 100 {
		t.Fatalf("unexpected close expired request: %+v", fakeRPC.lastCloseExpiredOrdersReq)
	}
}

func TestRunOncePropagatesRPCFailure(t *testing.T) {
	fakeRPC := &fakeJobOrderRPC{
		closeExpiredOrdersErr: errors.New("rpc failed"),
	}
	l := NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config: config.Config{
			Interval:  time.Minute,
			BatchSize: 100,
		},
		OrderRpc: fakeRPC,
	})

	if err := l.RunOnce(); err == nil {
		t.Fatalf("expected rpc error")
	}
}
