package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"damai-go/jobs/order-close-worker/internal/svc"
	"damai-go/services/order-rpc/closequeue"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
)

type fakeOrderVerifyRPC struct {
	verifyAttemptDueResp  *orderrpc.BoolResp
	verifyAttemptDueErr   error
	verifyAttemptDueCalls int
	lastVerifyAttemptReq  *orderrpc.VerifyAttemptDueReq
}

func (f *fakeOrderVerifyRPC) CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq) (*orderrpc.BoolResp, error) {
	return &orderrpc.BoolResp{Success: true}, nil
}

func (f *fakeOrderVerifyRPC) VerifyAttemptDue(ctx context.Context, in *orderrpc.VerifyAttemptDueReq) (*orderrpc.BoolResp, error) {
	f.verifyAttemptDueCalls++
	f.lastVerifyAttemptReq = in
	if f.verifyAttemptDueResp == nil {
		f.verifyAttemptDueResp = &orderrpc.BoolResp{Success: true}
	}
	return f.verifyAttemptDueResp, f.verifyAttemptDueErr
}

func TestVerifyAttemptDueTaskLogicHandleCallsOrderRPC(t *testing.T) {
	dueAt := time.Date(2026, time.March, 30, 19, 45, 0, 0, time.Local)
	body, err := closequeue.MarshalVerifyAttemptPayload(91001, 10001, dueAt)
	if err != nil {
		t.Fatalf("MarshalVerifyAttemptPayload returned error: %v", err)
	}

	orderRPC := &fakeOrderVerifyRPC{}
	l := NewVerifyAttemptDueTaskLogic(&svc.ServiceContext{OrderRpc: orderRPC})

	err = l.Handle(context.Background(), asynq.NewTask(closequeue.TaskTypeVerifyAttemptDue, body))
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if orderRPC.verifyAttemptDueCalls != 1 {
		t.Fatalf("expected one VerifyAttemptDue call, got %d", orderRPC.verifyAttemptDueCalls)
	}
	if orderRPC.lastVerifyAttemptReq == nil {
		t.Fatalf("expected verify attempt request to be recorded")
	}
	if orderRPC.lastVerifyAttemptReq.OrderNumber != 91001 {
		t.Fatalf("expected order number 91001, got %+v", orderRPC.lastVerifyAttemptReq)
	}
	if orderRPC.lastVerifyAttemptReq.DueAtUnix != dueAt.Unix() {
		t.Fatalf("expected dueAtUnix %d, got %+v", dueAt.Unix(), orderRPC.lastVerifyAttemptReq)
	}
}

func TestVerifyAttemptDueTaskLogicHandleSkipsRetryOnBadPayload(t *testing.T) {
	l := NewVerifyAttemptDueTaskLogic(&svc.ServiceContext{OrderRpc: &fakeOrderVerifyRPC{}})

	err := l.Handle(context.Background(), asynq.NewTask(closequeue.TaskTypeVerifyAttemptDue, []byte("{bad json")))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry, got %v", err)
	}
}
