package svc

import (
	"context"
	"testing"
	"time"

	"damai-go/pkg/xredis"
	"damai-go/services/order-rpc/closequeue"
	"damai-go/services/order-rpc/internal/config"

	"github.com/hibiken/asynq"
)

type fakeAsynqEnqueuer struct {
	enqueueErr   error
	enqueueCalls int
	lastTask     *asynq.Task
}

func (f *fakeAsynqEnqueuer) EnqueueContext(_ context.Context, task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.enqueueCalls++
	f.lastTask = task
	return &asynq.TaskInfo{}, f.enqueueErr
}

func TestAsynqAsyncCloseClientEnqueueCloseTimeout(t *testing.T) {
	expireAt := time.Date(2026, time.March, 29, 19, 45, 0, 0, time.UTC)
	enqueuer := &fakeAsynqEnqueuer{}
	client := &asynqAsyncCloseClient{
		enqueuer:  enqueuer,
		queue:     "order_close",
		maxRetry:  8,
		uniqueTTL: 30 * time.Minute,
	}

	err := client.EnqueueCloseTimeout(context.Background(), 91001, expireAt)
	if err != nil {
		t.Fatalf("EnqueueCloseTimeout returned error: %v", err)
	}
	if enqueuer.enqueueCalls != 1 {
		t.Fatalf("expected enqueue once, got %d", enqueuer.enqueueCalls)
	}
	if enqueuer.lastTask == nil {
		t.Fatalf("expected task to be enqueued")
	}
	if enqueuer.lastTask.Type() != closequeue.TaskTypeCloseTimeout {
		t.Fatalf("expected task type %s, got %s", closequeue.TaskTypeCloseTimeout, enqueuer.lastTask.Type())
	}
	payload, err := closequeue.ParseCloseTimeoutPayload(enqueuer.lastTask.Payload())
	if err != nil {
		t.Fatalf("ParseCloseTimeoutPayload returned error: %v", err)
	}
	if payload.OrderNumber != 91001 {
		t.Fatalf("expected order number 91001, got %d", payload.OrderNumber)
	}
}

func TestAsynqAsyncCloseClientEnqueueVerifyAttemptDue(t *testing.T) {
	dueAt := time.Date(2026, time.April, 5, 20, 15, 0, 0, time.UTC)
	enqueuer := &fakeAsynqEnqueuer{}
	client := &asynqAsyncCloseClient{
		enqueuer:  enqueuer,
		queue:     "order_close",
		maxRetry:  8,
		uniqueTTL: 30 * time.Minute,
	}

	err := client.EnqueueVerifyAttemptDue(context.Background(), 91001, 10001, dueAt)
	if err != nil {
		t.Fatalf("EnqueueVerifyAttemptDue returned error: %v", err)
	}
	if enqueuer.enqueueCalls != 1 {
		t.Fatalf("expected enqueue once, got %d", enqueuer.enqueueCalls)
	}
	if enqueuer.lastTask == nil {
		t.Fatalf("expected task to be enqueued")
	}
	if enqueuer.lastTask.Type() != closequeue.TaskTypeVerifyAttemptDue {
		t.Fatalf("expected task type %s, got %s", closequeue.TaskTypeVerifyAttemptDue, enqueuer.lastTask.Type())
	}
	payload, err := closequeue.ParseVerifyAttemptPayload(enqueuer.lastTask.Payload())
	if err != nil {
		t.Fatalf("ParseVerifyAttemptPayload returned error: %v", err)
	}
	if payload.OrderNumber != 91001 || payload.ProgramID != 10001 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestNewAsyncCloseClientReturnsNilWhenDisabled(t *testing.T) {
	client, err := newAsyncCloseClient(config.AsyncCloseConfig{})
	if err != nil {
		t.Fatalf("newAsyncCloseClient returned error: %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client when async close disabled")
	}
}

func TestNewAsyncCloseClientReturnsClientWhenEnabled(t *testing.T) {
	client, err := newAsyncCloseClient(config.AsyncCloseConfig{
		Enable: true,
		Queue:  "order_close",
		Redis: xredis.Config{
			Host: "127.0.0.1:6379",
			Type: "node",
		},
	})
	if err != nil {
		t.Fatalf("newAsyncCloseClient returned error: %v", err)
	}
	if client == nil {
		t.Fatalf("expected non-nil client when async close enabled")
	}
}
