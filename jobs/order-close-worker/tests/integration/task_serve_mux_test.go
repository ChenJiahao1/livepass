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
	verifyAttemptDueReqs  []*orderrpc.VerifyAttemptDueReq
}

func (f *fakeWorkerOrderRPC) CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq) (*orderrpc.BoolResp, error) {
	f.closeExpiredOrderReqs = append(f.closeExpiredOrderReqs, in)
	return &orderrpc.BoolResp{Success: true}, nil
}

func (f *fakeWorkerOrderRPC) VerifyAttemptDue(ctx context.Context, in *orderrpc.VerifyAttemptDueReq) (*orderrpc.BoolResp, error) {
	f.verifyAttemptDueReqs = append(f.verifyAttemptDueReqs, in)
	return &orderrpc.BoolResp{Success: true}, nil
}

func TestNewServeMuxRoutesCloseTimeoutAndVerifyAttemptTasks(t *testing.T) {
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

	verifyDueAt := time.Date(2026, time.March, 29, 21, 0, 30, 0, time.Local)
	verifyBody, err := closequeue.MarshalVerifyAttemptPayload(92002, 10001, verifyDueAt)
	if err != nil {
		t.Fatalf("MarshalVerifyAttemptPayload returned error: %v", err)
	}
	if err := mux.ProcessTask(context.Background(), asynq.NewTask(closequeue.TaskTypeVerifyAttemptDue, verifyBody)); err != nil {
		t.Fatalf("ProcessTask(verify attempt) returned error: %v", err)
	}

	if len(fakeRPC.closeExpiredOrderReqs) != 1 || fakeRPC.closeExpiredOrderReqs[0].GetOrderNumber() != 92001 {
		t.Fatalf("unexpected close timeout requests: %+v", fakeRPC.closeExpiredOrderReqs)
	}
	if len(fakeRPC.verifyAttemptDueReqs) != 1 {
		t.Fatalf("unexpected verify attempt requests: %+v", fakeRPC.verifyAttemptDueReqs)
	}
	if fakeRPC.verifyAttemptDueReqs[0].GetOrderNumber() != 92002 {
		t.Fatalf("expected verify attempt order number 92002, got %+v", fakeRPC.verifyAttemptDueReqs[0])
	}
	if fakeRPC.verifyAttemptDueReqs[0].GetDueAtUnix() != verifyDueAt.Unix() {
		t.Fatalf("expected verify attempt dueAtUnix %d, got %+v", verifyDueAt.Unix(), fakeRPC.verifyAttemptDueReqs[0])
	}
}
