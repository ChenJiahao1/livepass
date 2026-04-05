package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"damai-go/jobs/order-rush-reconcile/internal/config"
	logicpkg "damai-go/jobs/order-rush-reconcile/internal/logic"
	"damai-go/jobs/order-rush-reconcile/internal/svc"
	"damai-go/services/order-rpc/orderrpc"

	"google.golang.org/grpc"
)

type fakeOrderRushReconcileRPC struct {
	reconcileResp *orderrpc.ReconcileRushAttemptsResp
	reconcileErr  error
	reconcileReqs []*orderrpc.ReconcileRushAttemptsReq
}

func (f *fakeOrderRushReconcileRPC) CreatePurchaseToken(ctx context.Context, in *orderrpc.CreatePurchaseTokenReq, opts ...grpc.CallOption) (*orderrpc.CreatePurchaseTokenResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) CreateOrder(ctx context.Context, in *orderrpc.CreateOrderReq, opts ...grpc.CallOption) (*orderrpc.CreateOrderResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) PollOrderProgress(ctx context.Context, in *orderrpc.PollOrderProgressReq, opts ...grpc.CallOption) (*orderrpc.PollOrderProgressResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) ListOrders(ctx context.Context, in *orderrpc.ListOrdersReq, opts ...grpc.CallOption) (*orderrpc.ListOrdersResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) GetOrder(ctx context.Context, in *orderrpc.GetOrderReq, opts ...grpc.CallOption) (*orderrpc.OrderDetailInfo, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) GetOrderCache(ctx context.Context, in *orderrpc.GetOrderCacheReq, opts ...grpc.CallOption) (*orderrpc.GetOrderCacheResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) GetOrderServiceView(ctx context.Context, in *orderrpc.GetOrderServiceViewReq, opts ...grpc.CallOption) (*orderrpc.OrderServiceViewResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) CancelOrder(ctx context.Context, in *orderrpc.CancelOrderReq, opts ...grpc.CallOption) (*orderrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) PayOrder(ctx context.Context, in *orderrpc.PayOrderReq, opts ...grpc.CallOption) (*orderrpc.PayOrderResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) PayCheck(ctx context.Context, in *orderrpc.PayCheckReq, opts ...grpc.CallOption) (*orderrpc.PayCheckResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) PreviewRefundOrder(ctx context.Context, in *orderrpc.PreviewRefundOrderReq, opts ...grpc.CallOption) (*orderrpc.PreviewRefundOrderResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) RefundOrder(ctx context.Context, in *orderrpc.RefundOrderReq, opts ...grpc.CallOption) (*orderrpc.RefundOrderResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq, opts ...grpc.CallOption) (*orderrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) CloseExpiredOrders(ctx context.Context, in *orderrpc.CloseExpiredOrdersReq, opts ...grpc.CallOption) (*orderrpc.CloseExpiredOrdersResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) VerifyAttemptDue(ctx context.Context, in *orderrpc.VerifyAttemptDueReq, opts ...grpc.CallOption) (*orderrpc.BoolResp, error) {
	return nil, nil
}

func (f *fakeOrderRushReconcileRPC) ReconcileRushAttempts(ctx context.Context, in *orderrpc.ReconcileRushAttemptsReq, opts ...grpc.CallOption) (*orderrpc.ReconcileRushAttemptsResp, error) {
	f.reconcileReqs = append(f.reconcileReqs, in)
	return f.reconcileResp, f.reconcileErr
}

func (f *fakeOrderRushReconcileRPC) CountActiveTicketsByUserProgram(ctx context.Context, in *orderrpc.CountActiveTicketsByUserProgramReq, opts ...grpc.CallOption) (*orderrpc.CountActiveTicketsByUserProgramResp, error) {
	return nil, nil
}

func TestRunOnceForwardsBatchLimit(t *testing.T) {
	fakeRPC := &fakeOrderRushReconcileRPC{
		reconcileResp: &orderrpc.ReconcileRushAttemptsResp{ReconciledCount: 3},
	}
	l := logicpkg.NewReconcileRushAttemptsLogic(context.Background(), &svc.ServiceContext{
		Config: config.Config{
			Interval:  time.Second,
			BatchSize: 80,
		},
		OrderRpc: fakeRPC,
	})

	if err := l.RunOnce(); err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if len(fakeRPC.reconcileReqs) != 1 {
		t.Fatalf("reconcile requests = %d, want 1", len(fakeRPC.reconcileReqs))
	}
	if fakeRPC.reconcileReqs[0] == nil || fakeRPC.reconcileReqs[0].GetLimit() != 80 {
		t.Fatalf("unexpected reconcile request: %+v", fakeRPC.reconcileReqs[0])
	}
}

func TestRunOncePropagatesRPCFailure(t *testing.T) {
	fakeRPC := &fakeOrderRushReconcileRPC{
		reconcileErr: errors.New("rpc failed"),
	}
	l := logicpkg.NewReconcileRushAttemptsLogic(context.Background(), &svc.ServiceContext{
		Config: config.Config{
			Interval:  time.Second,
			BatchSize: 80,
		},
		OrderRpc: fakeRPC,
	})

	if err := l.RunOnce(); err == nil {
		t.Fatalf("expected rpc error")
	}
}
