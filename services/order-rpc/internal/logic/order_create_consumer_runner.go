package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

func StartOrderCreateConsumer(ctx context.Context, svcCtx *svc.ServiceContext) func() {
	if svcCtx == nil || svcCtx.OrderCreateConsumer == nil {
		return func() {}
	}

	consumeCtx, cancel := context.WithCancel(ctx)
	consumerLogic := NewCreateOrderConsumerLogic(consumeCtx, svcCtx)

	go func() {
		if err := svcCtx.OrderCreateConsumer.Start(consumeCtx, func(_ context.Context, body []byte) error {
			return consumerLogic.Consume(body)
		}); err != nil {
			logx.WithContext(consumeCtx).Errorf("order create consumer exited: %v", err)
		}
	}()

	return func() {
		cancel()
		_ = svcCtx.OrderCreateConsumer.Close()
	}
}
