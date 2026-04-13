package worker

import (
	"context"

	"damai-go/jobs/order-close/internal/svc"
	"damai-go/jobs/order-close/taskdef"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
)

type CloseTimeoutTaskLogic struct {
	svcCtx *svc.WorkerServiceContext
}

func NewCloseTimeoutTaskLogic(svcCtx *svc.WorkerServiceContext) *CloseTimeoutTaskLogic {
	return &CloseTimeoutTaskLogic{svcCtx: svcCtx}
}

func (l *CloseTimeoutTaskLogic) Handle(ctx context.Context, task *asynq.Task) error {
	payload, err := taskdef.Parse(task.Payload())
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
