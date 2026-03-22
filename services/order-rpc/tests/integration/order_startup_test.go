package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	logicpkg "damai-go/services/order-rpc/internal/logic"
)

func TestKafkaConsumerStartAndClose(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	consumer, ok := svcCtx.OrderCreateConsumer.(*fakeOrderCreateConsumer)
	if !ok {
		t.Fatalf("expected fake order create consumer, got %T", svcCtx.OrderCreateConsumer)
	}

	stop := logicpkg.StartOrderCreateConsumer(context.Background(), svcCtx)
	select {
	case <-consumer.started:
	case <-time.After(time.Second):
		t.Fatalf("expected consumer to start")
	}
	if consumer.startCalls != 1 {
		t.Fatalf("expected consumer start once, got %d", consumer.startCalls)
	}
	if consumer.handler == nil {
		t.Fatalf("expected consumer handler to be registered")
	}

	stop()
	if consumer.closeCalls != 1 {
		t.Fatalf("expected consumer close once, got %d", consumer.closeCalls)
	}
}

func TestKafkaConsumerRestartsAfterRecoverableStartError(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	consumer, ok := svcCtx.OrderCreateConsumer.(*fakeOrderCreateConsumer)
	if !ok {
		t.Fatalf("expected fake order create consumer, got %T", svcCtx.OrderCreateConsumer)
	}
	consumer.startErrs = []error{errors.New("temporary kafka error"), nil}

	stop := logicpkg.StartOrderCreateConsumer(context.Background(), svcCtx)
	defer stop()

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if consumer.startCalls >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected consumer to restart after error, got startCalls=%d", consumer.startCalls)
}
