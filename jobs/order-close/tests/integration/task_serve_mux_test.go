package integration_test

import (
	"context"
	"testing"

	"damai-go/jobs/order-close/internal/svc"
	workerpkg "damai-go/jobs/order-close/internal/worker"
	"damai-go/jobs/order-close/taskdef"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
	"google.golang.org/grpc"
)

type fakeMergedWorkerOrderRPC struct {
	closeExpiredOrderReqs []*orderrpc.CloseExpiredOrderReq
}

func (f *fakeMergedWorkerOrderRPC) CloseExpiredOrder(_ context.Context, in *orderrpc.CloseExpiredOrderReq, _ ...grpc.CallOption) (*orderrpc.BoolResp, error) {
	f.closeExpiredOrderReqs = append(f.closeExpiredOrderReqs, in)
	return &orderrpc.BoolResp{Success: true}, nil
}

func TestWorkerServeMuxRoutesCloseTimeoutTask(t *testing.T) {
	body, err := taskdef.Marshal(92001)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	orderRPC := &fakeMergedWorkerOrderRPC{}
	mux := workerpkg.NewServeMux(&svc.WorkerServiceContext{
		OrderRpc: orderRPC,
	})

	if err := mux.ProcessTask(context.Background(), asynq.NewTask(taskdef.TaskTypeCloseTimeout, body)); err != nil {
		t.Fatalf("ProcessTask returned error: %v", err)
	}
	if len(orderRPC.closeExpiredOrderReqs) != 1 || orderRPC.closeExpiredOrderReqs[0].GetOrderNumber() != 92001 {
		t.Fatalf("unexpected close expired order reqs: %+v", orderRPC.closeExpiredOrderReqs)
	}
}
