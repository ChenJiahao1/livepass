package logic

import (
	"context"
	"time"

	"damai-go/services/order-rpc/internal/mq"
	"damai-go/services/order-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

func StartOrderCreateConsumer(ctx context.Context, svcCtx *svc.ServiceContext) func() {
	if svcCtx == nil || svcCtx.OrderCreateConsumerFactory == nil {
		return func() {}
	}

	consumeCtx, cancel := context.WithCancel(ctx)
	consumerLogic := NewCreateOrderConsumerLogic(consumeCtx, svcCtx)
	retryBackoff := svcCtx.Config.Kafka.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = time.Second
	}
	workers := svcCtx.Config.Kafka.ConsumerWorkers
	if workers <= 0 {
		workers = 1
	}

	consumers := make([]mq.OrderCreateConsumer, 0, workers)
	for workerID := 0; workerID < workers; workerID++ {
		consumer := svcCtx.OrderCreateConsumerFactory.New(svcCtx.Config.Kafka)
		if consumer == nil {
			continue
		}
		consumers = append(consumers, consumer)

		go func(workerID int, consumer mq.OrderCreateConsumer) {
			for {
				err := consumer.Start(consumeCtx, func(_ context.Context, body []byte) error {
					return consumerLogic.Consume(body)
				})
				if consumeCtx.Err() != nil {
					return
				}
				if err == nil {
					logx.WithContext(consumeCtx).Errorf(
						"order create consumer worker=%d exited unexpectedly, restarting in %s",
						workerID,
						retryBackoff,
					)
				} else {
					logx.WithContext(consumeCtx).Errorf(
						"order create consumer worker=%d exited: %v, restarting in %s",
						workerID,
						err,
						retryBackoff,
					)
				}

				timer := time.NewTimer(retryBackoff)
				select {
				case <-consumeCtx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					return
				case <-timer.C:
				}
			}
		}(workerID, consumer)
	}

	return func() {
		cancel()
		for _, consumer := range consumers {
			_ = consumer.Close()
		}
	}
}
