package logic

import (
	"context"

	"damai-go/jobs/order-close-worker/internal/svc"
	"damai-go/services/order-rpc/closequeue"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
)

type CloseTimeoutTaskLogic struct {
	svcCtx *svc.ServiceContext
}

func NewCloseTimeoutTaskLogic(svcCtx *svc.ServiceContext) *CloseTimeoutTaskLogic {
	return &CloseTimeoutTaskLogic{svcCtx: svcCtx}
}

func (l *CloseTimeoutTaskLogic) Handle(ctx context.Context, task *asynq.Task) error {
	payload, err := closequeue.ParseCloseTimeoutPayload(task.Payload())
	if err != nil {
		return asynq.SkipRetry
	}
	if l.svcCtx == nil || l.svcCtx.OrderRpc == nil {
		return asynq.SkipRetry
	}

	_, err = l.svcCtx.OrderRpc.CloseExpiredOrder(ctx, &orderrpc.CloseExpiredOrderReq{
		OrderNumber: payload.OrderNumber,
	})
	return err
}
