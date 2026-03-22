package logic

import (
	"context"
	"time"

	"damai-go/services/order-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

func StartOrderCreateConsumer(ctx context.Context, svcCtx *svc.ServiceContext) func() {
	if svcCtx == nil || svcCtx.OrderCreateConsumer == nil {
		return func() {}
	}

	consumeCtx, cancel := context.WithCancel(ctx)
	consumerLogic := NewCreateOrderConsumerLogic(consumeCtx, svcCtx)
	retryBackoff := svcCtx.Config.Kafka.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = time.Second
	}

	go func() {
		for {
			err := svcCtx.OrderCreateConsumer.Start(consumeCtx, func(_ context.Context, body []byte) error {
				return consumerLogic.Consume(body)
			})
			if consumeCtx.Err() != nil {
				return
			}
			if err == nil {
				logx.WithContext(consumeCtx).Errorf("order create consumer exited unexpectedly, restarting in %s", retryBackoff)
			} else {
				logx.WithContext(consumeCtx).Errorf("order create consumer exited: %v, restarting in %s", err, retryBackoff)
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
	}()

	return func() {
		cancel()
		_ = svcCtx.OrderCreateConsumer.Close()
	}
}
