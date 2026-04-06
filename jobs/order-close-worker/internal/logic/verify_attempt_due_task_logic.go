package logic

import (
	"context"
	"time"

	"damai-go/jobs/order-close-worker/internal/svc"
	"damai-go/services/order-rpc/closequeue"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/hibiken/asynq"
)

const taskPayloadTimeLayout = "2006-01-02 15:04:05"

type VerifyAttemptDueTaskLogic struct {
	svcCtx *svc.ServiceContext
}

func NewVerifyAttemptDueTaskLogic(svcCtx *svc.ServiceContext) *VerifyAttemptDueTaskLogic {
	return &VerifyAttemptDueTaskLogic{svcCtx: svcCtx}
}

func (l *VerifyAttemptDueTaskLogic) Handle(ctx context.Context, task *asynq.Task) error {
	payload, err := closequeue.ParseVerifyAttemptPayload(task.Payload())
	if err != nil {
		return asynq.SkipRetry
	}
	if l.svcCtx == nil || l.svcCtx.OrderRpc == nil {
		return asynq.SkipRetry
	}

	dueAt, err := time.ParseInLocation(taskPayloadTimeLayout, payload.DueAt, time.Local)
	if err != nil {
		return asynq.SkipRetry
	}

	_, err = l.svcCtx.OrderRpc.VerifyAttemptDue(ctx, &orderrpc.VerifyAttemptDueReq{
		OrderNumber: payload.OrderNumber,
		DueAtUnix:   dueAt.Unix(),
	})
	return err
}
