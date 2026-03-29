package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"damai-go/jobs/order-close/internal/config"
	logicpkg "damai-go/jobs/order-close/internal/logic"
	"damai-go/jobs/order-close/internal/svc"
	"damai-go/services/order-rpc/orderrpc"

	"google.golang.org/grpc"
)

type fakeJobOrderRPC struct {
	closeExpiredOrdersResp *orderrpc.CloseExpiredOrdersResp
	closeExpiredOrdersErr  error
	closeExpiredOrdersReqs []*orderrpc.CloseExpiredOrdersReq
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

func (f *fakeJobOrderRPC) GetOrderCache(ctx context.Context, in *orderrpc.GetOrderCacheReq, opts ...grpc.CallOption) (*orderrpc.GetOrderCacheResp, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) GetOrderServiceView(ctx context.Context, in *orderrpc.GetOrderServiceViewReq, opts ...grpc.CallOption) (*orderrpc.OrderServiceViewResp, error) {
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

func (f *fakeJobOrderRPC) PreviewRefundOrder(ctx context.Context, in *orderrpc.PreviewRefundOrderReq, opts ...grpc.CallOption) (*orderrpc.PreviewRefundOrderResp, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) RefundOrder(ctx context.Context, in *orderrpc.RefundOrderReq, opts ...grpc.CallOption) (*orderrpc.RefundOrderResp, error) {
	return nil, nil
}

func (f *fakeJobOrderRPC) CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq, opts ...grpc.CallOption) (*orderrpc.BoolResp, error) {
	return &orderrpc.BoolResp{Success: true}, nil
}

func (f *fakeJobOrderRPC) CloseExpiredOrders(ctx context.Context, in *orderrpc.CloseExpiredOrdersReq, opts ...grpc.CallOption) (*orderrpc.CloseExpiredOrdersResp, error) {
	f.closeExpiredOrdersReqs = append(f.closeExpiredOrdersReqs, in)
	return f.closeExpiredOrdersResp, f.closeExpiredOrdersErr
}

func (f *fakeJobOrderRPC) CountActiveTicketsByUserProgram(ctx context.Context, in *orderrpc.CountActiveTicketsByUserProgramReq, opts ...grpc.CallOption) (*orderrpc.CountActiveTicketsByUserProgramResp, error) {
	return nil, nil
}

func TestRunOnceForwardsBatchLimit(t *testing.T) {
	fakeRPC := &fakeJobOrderRPC{
		closeExpiredOrdersResp: &orderrpc.CloseExpiredOrdersResp{ClosedCount: 3},
	}
	l := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config: config.Config{
			Interval:  time.Minute,
			BatchSize: 100,
		},
		OrderRpc: fakeRPC,
	})

	if err := l.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(fakeRPC.closeExpiredOrdersReqs) != 1 {
		t.Fatalf("close expired requests = %d, want 1", len(fakeRPC.closeExpiredOrdersReqs))
	}
	if fakeRPC.closeExpiredOrdersReqs[0] == nil || fakeRPC.closeExpiredOrdersReqs[0].Limit != 100 {
		t.Fatalf("unexpected close expired request: %+v", fakeRPC.closeExpiredOrdersReqs[0])
	}
}

func TestRunOncePropagatesRPCFailure(t *testing.T) {
	fakeRPC := &fakeJobOrderRPC{
		closeExpiredOrdersErr: errors.New("rpc failed"),
	}
	l := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
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

func TestRunOnceForwardsSlotWindowAndAdvancesCheckpoint(t *testing.T) {
	fakeRPC := &fakeJobOrderRPC{
		closeExpiredOrdersResp: &orderrpc.CloseExpiredOrdersResp{ClosedCount: 2},
	}
	l := logicpkg.NewCloseExpiredOrdersLogic(context.Background(), &svc.ServiceContext{
		Config: config.Config{
			Interval:          time.Minute,
			BatchSize:         100,
			ScanSlotStart:     10,
			ScanSlotEnd:       13,
			ScanSlotBatchSize: 2,
			CheckpointSlot:    10,
		},
		OrderRpc: fakeRPC,
	})

	if err := l.RunOnce(); err != nil {
		t.Fatalf("first RunOnce returned error: %v", err)
	}
	if err := l.RunOnce(); err != nil {
		t.Fatalf("second RunOnce returned error: %v", err)
	}
	if err := l.RunOnce(); err != nil {
		t.Fatalf("third RunOnce returned error: %v", err)
	}

	if len(fakeRPC.closeExpiredOrdersReqs) != 3 {
		t.Fatalf("close expired requests = %d, want 3", len(fakeRPC.closeExpiredOrdersReqs))
	}

	req0 := fakeRPC.closeExpiredOrdersReqs[0]
	if req0.GetLogicSlotStart() != 10 || req0.GetLogicSlotCount() != 2 {
		t.Fatalf("first request = %+v, want start=10 count=2", req0)
	}

	req1 := fakeRPC.closeExpiredOrdersReqs[1]
	if req1.GetLogicSlotStart() != 12 || req1.GetLogicSlotCount() != 2 {
		t.Fatalf("second request = %+v, want start=12 count=2", req1)
	}

	req2 := fakeRPC.closeExpiredOrdersReqs[2]
	if req2.GetLogicSlotStart() != 10 || req2.GetLogicSlotCount() != 2 {
		t.Fatalf("third request = %+v, want start=10 count=2", req2)
	}
}
