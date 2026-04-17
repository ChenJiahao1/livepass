package worker

import (
	"context"
	"errors"
	"time"

	"livepass/jobs/rush-inventory-preheat/internal/svc"
	"livepass/jobs/rush-inventory-preheat/taskdef"
	"livepass/pkg/xerr"
	orderrpc "livepass/services/order-rpc/orderrpc"
	programrpc "livepass/services/program-rpc/programrpc"

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

	showTimes, err := l.svcCtx.ShowTimeStore.ListByProgramID(ctx, payload.ProgramId)
	if err != nil {
		if errors.Is(err, xerr.ErrProgramShowTimeNotFound) {
			return nil
		}
		return err
	}

	matchedShowTimeIDs := make([]int64, 0, len(showTimes))
	for _, showTime := range showTimes {
		if showTime == nil || !showTime.RushSaleOpenTime.Valid {
			continue
		}
		if showTime.RushSaleOpenTime.Time.Format(rushInventoryPreheatTimeLayout) != expectedOpenTime.Format(rushInventoryPreheatTimeLayout) {
			return nil
		}
		matchedShowTimeIDs = append(matchedShowTimeIDs, showTime.ID)
	}
	if len(matchedShowTimeIDs) == 0 {
		return nil
	}

	if _, err := l.svcCtx.OrderRpc.PrimeRushRuntime(ctx, &orderrpc.PrimeRushRuntimeReq{
		ProgramId: payload.ProgramId,
	}); err != nil {
		return err
	}
	for _, showTimeID := range matchedShowTimeIDs {
		if _, err := l.svcCtx.ProgramRpc.PrimeSeatLedger(ctx, &programrpc.PrimeSeatLedgerReq{
			ShowTimeId: showTimeID,
		}); err != nil {
			return err
		}
	}

	_, err = l.svcCtx.ShowTimeStore.MarkInventoryPreheatedByProgram(ctx, payload.ProgramId, expectedOpenTime, time.Now())
	return err
}
