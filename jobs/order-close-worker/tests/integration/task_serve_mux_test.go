package integration_test

import (
	"context"
	"testing"
	"time"

	workersvc "damai-go/jobs/order-close-worker/internal/svc"
	workerpkg "damai-go/jobs/order-close-worker/internal/worker"
	"damai-go/services/order-rpc/closequeue"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
)

type fakeWorkerOrderRPC struct {
	closeExpiredOrderReqs []*orderrpc.CloseExpiredOrderReq
}

func (f *fakeWorkerOrderRPC) CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq) (*orderrpc.BoolResp, error) {
	f.closeExpiredOrderReqs = append(f.closeExpiredOrderReqs, in)
	return &orderrpc.BoolResp{Success: true}, nil
}

func TestNewServeMuxRoutesCloseTimeoutOnly(t *testing.T) {
	fakeRPC := &fakeWorkerOrderRPC{}
	mux := workerpkg.NewServeMux(&workersvc.ServiceContext{
		OrderRpc: fakeRPC,
	})

	closeBody, err := closequeue.MarshalCloseTimeoutPayload(92001, time.Date(2026, time.March, 29, 21, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("MarshalCloseTimeoutPayload returned error: %v", err)
	}
	if err := mux.ProcessTask(context.Background(), asynq.NewTask(closequeue.TaskTypeCloseTimeout, closeBody)); err != nil {
		t.Fatalf("ProcessTask(close timeout) returned error: %v", err)
	}

	if len(fakeRPC.closeExpiredOrderReqs) != 1 || fakeRPC.closeExpiredOrderReqs[0].GetOrderNumber() != 92001 {
		t.Fatalf("unexpected close timeout requests: %+v", fakeRPC.closeExpiredOrderReqs)
	}
}
