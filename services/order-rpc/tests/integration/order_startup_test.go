package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	logicpkg "livepass/services/order-rpc/internal/logic"
)

func TestKafkaConsumerStartAndClose(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	factory, ok := svcCtx.OrderCreateConsumerFactory.(*fakeOrderCreateConsumerFactory)
	if !ok {
		t.Fatalf("expected fake order create consumer factory, got %T", svcCtx.OrderCreateConsumerFactory)
	}

	stop := logicpkg.StartOrderCreateConsumer(context.Background(), svcCtx)
	waitForConsumerWorkers(t, factory, 5)

	if factory.createCalls != 5 {
		t.Fatalf("expected 5 consumers, got %d", factory.createCalls)
	}
	for idx, consumer := range factory.consumers {
		if consumer.startCalls != 1 {
			t.Fatalf("expected consumer %d start once, got %d", idx, consumer.startCalls)
		}
		if consumer.handler == nil {
			t.Fatalf("expected consumer %d handler to be registered", idx)
		}
	}

	stop()
	if factory.closeCalls != 5 {
		t.Fatalf("expected 5 closes, got %d", factory.closeCalls)
	}
}

func TestKafkaConsumerRestartsAfterRecoverableStartError(t *testing.T) {
	svcCtx, _, _, _ := newOrderTestServiceContext(t)
	factory, ok := svcCtx.OrderCreateConsumerFactory.(*fakeOrderCreateConsumerFactory)
	if !ok {
		t.Fatalf("expected fake order create consumer factory, got %T", svcCtx.OrderCreateConsumerFactory)
	}
	factory.seedConsumers = []*fakeOrderCreateConsumer{
		{startErrs: []error{errors.New("temporary kafka error"), nil}, started: make(chan struct{}, 1)},
	}

	stop := logicpkg.StartOrderCreateConsumer(context.Background(), svcCtx)
	defer stop()

	waitForConsumerWorkers(t, factory, 5)

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(factory.consumers) > 0 && factory.consumers[0].startCalls >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("expected worker restart after recoverable error")
}

func waitForConsumerWorkers(t *testing.T, factory *fakeOrderCreateConsumerFactory, workers int) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if factory.createCalls == workers && len(factory.consumers) == workers {
			allStarted := true
			for _, consumer := range factory.consumers {
				if consumer.startCalls == 0 {
					allStarted = false
					break
				}
			}
			if allStarted {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("expected %d consumer workers to start, got createCalls=%d consumers=%d", workers, factory.createCalls, len(factory.consumers))
}
