package worker

import (
	"context"
	"errors"
	"time"

	"damai-go/jobs/rush-inventory-preheat/internal/svc"
	"damai-go/jobs/rush-inventory-preheat/taskdef"
	"damai-go/pkg/xerr"
	orderrpc "damai-go/services/order-rpc/orderrpc"
	programrpc "damai-go/services/program-rpc/programrpc"

	"github.com/hibiken/asynq"
)

const rushInventoryPreheatTimeLayout = "2006-01-02 15:04:05"

type RushInventoryPreheatTaskLogic struct {
	svcCtx *svc.WorkerServiceContext
}

func NewRushInventoryPreheatTaskLogic(svcCtx *svc.WorkerServiceContext) *RushInventoryPreheatTaskLogic {
	return &RushInventoryPreheatTaskLogic{svcCtx: svcCtx}
}

func (l *RushInventoryPreheatTaskLogic) Handle(ctx context.Context, task *asynq.Task) error {
	payload, err := taskdef.Parse(task.Payload())
	if err != nil {
		return asynq.SkipRetry
	}
	expectedOpenTime, err := time.ParseInLocation(rushInventoryPreheatTimeLayout, payload.ExpectedRushSaleOpenTime, time.Local)
	if err != nil {
		return asynq.SkipRetry
	}
	if l.svcCtx == nil || l.svcCtx.ShowTimeStore == nil || l.svcCtx.OrderRpc == nil || l.svcCtx.ProgramRpc == nil {
		return asynq.SkipRetry
	}

	showTime, err := l.svcCtx.ShowTimeStore.FindOne(ctx, payload.ShowTimeId)
	if err != nil {
		if errors.Is(err, xerr.ErrProgramShowTimeNotFound) {
			return nil
		}
		return err
	}
	if showTime == nil || !showTime.RushSaleOpenTime.Valid {
		return nil
	}
	if showTime.RushSaleOpenTime.Time.Format(rushInventoryPreheatTimeLayout) != expectedOpenTime.Format(rushInventoryPreheatTimeLayout) {
		return nil
	}

	if _, err := l.svcCtx.OrderRpc.PrimeAdmissionQuota(ctx, &orderrpc.PrimeAdmissionQuotaReq{
		ShowTimeId: payload.ShowTimeId,
	}); err != nil {
		return err
	}
	if _, err := l.svcCtx.ProgramRpc.PrimeSeatLedger(ctx, &programrpc.PrimeSeatLedgerReq{
		ShowTimeId: payload.ShowTimeId,
	}); err != nil {
		return err
	}

	_, err = l.svcCtx.ShowTimeStore.MarkInventoryPreheated(ctx, payload.ShowTimeId, expectedOpenTime, time.Now())
	return err
}
