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

type fakeOrderCloseRPC struct {
	closeExpiredOrderResp  *orderrpc.BoolResp
	closeExpiredOrderErr   error
	closeExpiredOrderCalls int
	lastCloseExpiredOrderReq *orderrpc.CloseExpiredOrderReq
}

func (f *fakeOrderCloseRPC) CloseExpiredOrder(ctx context.Context, in *orderrpc.CloseExpiredOrderReq) (*orderrpc.BoolResp, error) {
	f.closeExpiredOrderCalls++
	f.lastCloseExpiredOrderReq = in
	if f.closeExpiredOrderResp == nil {
		f.closeExpiredOrderResp = &orderrpc.BoolResp{Success: true}
	}
	return f.closeExpiredOrderResp, f.closeExpiredOrderErr
}

func TestCloseTimeoutTaskLogicHandleCallsOrderRPC(t *testing.T) {
	body, err := closequeue.MarshalCloseTimeoutPayload(91001, time.Date(2026, time.March, 29, 19, 45, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("MarshalCloseTimeoutPayload returned error: %v", err)
	}

	orderRPC := &fakeOrderCloseRPC{}
	l := NewCloseTimeoutTaskLogic(&svc.ServiceContext{OrderRpc: orderRPC})

	err = l.Handle(context.Background(), asynq.NewTask(closequeue.TaskTypeCloseTimeout, body))
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if orderRPC.closeExpiredOrderCalls != 1 {
		t.Fatalf("expected one CloseExpiredOrder call, got %d", orderRPC.closeExpiredOrderCalls)
	}
	if orderRPC.lastCloseExpiredOrderReq == nil || orderRPC.lastCloseExpiredOrderReq.OrderNumber != 91001 {
		t.Fatalf("expected order rpc to be called with order number 91001, got %+v", orderRPC.lastCloseExpiredOrderReq)
	}
}

func TestCloseTimeoutTaskLogicHandleSkipsRetryOnBadPayload(t *testing.T) {
	l := NewCloseTimeoutTaskLogic(&svc.ServiceContext{OrderRpc: &fakeOrderCloseRPC{}})

	err := l.Handle(context.Background(), asynq.NewTask(closequeue.TaskTypeCloseTimeout, []byte("{bad json")))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry, got %v", err)
	}
}
