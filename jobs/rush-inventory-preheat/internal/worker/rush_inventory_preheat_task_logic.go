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
	"github.com/zeromicro/go-zero/core/logx"
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
	taskKey := taskdef.TaskKey(payload.ProgramId, expectedOpenTime)

	showTimes, err := l.svcCtx.ShowTimeStore.ListByProgramID(ctx, payload.ProgramId)
	if err != nil {
		if errors.Is(err, xerr.ErrProgramShowTimeNotFound) {
			return markRushInventoryPreheatProcessed(ctx, l.svcCtx.ShowTimeStore, taskKey, "program_show_time_not_found")
		}
		return err
	}

	matchedShowTimeIDs := make([]int64, 0, len(showTimes))
	for _, showTime := range showTimes {
		if showTime == nil || !showTime.RushSaleOpenTime.Valid {
			continue
		}
		if showTime.RushSaleOpenTime.Time.Format(rushInventoryPreheatTimeLayout) != expectedOpenTime.Format(rushInventoryPreheatTimeLayout) {
			return markRushInventoryPreheatProcessed(ctx, l.svcCtx.ShowTimeStore, taskKey, "rush_open_time_mismatch")
		}
		matchedShowTimeIDs = append(matchedShowTimeIDs, showTime.ID)
	}
	if len(matchedShowTimeIDs) == 0 {
		return markRushInventoryPreheatProcessed(ctx, l.svcCtx.ShowTimeStore, taskKey, "no_matched_show_time")
	}

	if _, err := l.svcCtx.OrderRpc.PrimeRushRuntime(ctx, &orderrpc.PrimeRushRuntimeReq{
		ProgramId: payload.ProgramId,
	}); err != nil {
		return markRushInventoryPreheatFailed(ctx, l.svcCtx.ShowTimeStore, taskKey, err)
	}
	for _, showTimeID := range matchedShowTimeIDs {
		if _, err := l.svcCtx.ProgramRpc.PrimeSeatLedger(ctx, &programrpc.PrimeSeatLedgerReq{
			ShowTimeId: showTimeID,
		}); err != nil {
			return markRushInventoryPreheatFailed(ctx, l.svcCtx.ShowTimeStore, taskKey, err)
		}
	}

	_, fromStatus, consumeAttempts, err := l.svcCtx.ShowTimeStore.MarkInventoryPreheatedByProgramAndTaskProcessed(
		ctx,
		payload.ProgramId,
		expectedOpenTime,
		taskdef.TaskTypeRushInventoryPreheat,
		taskKey,
		time.Now(),
	)
	if err == nil {
		logx.WithContext(ctx).Infow("delay_task_consume_state_transition",
			logx.Field("task_type", taskdef.TaskTypeRushInventoryPreheat),
			logx.Field("task_key", taskKey),
			logx.Field("from_status", fromStatus),
			logx.Field("to_status", 3),
			logx.Field("consume_attempts", consumeAttempts),
		)
	}
	return err
}

func markRushInventoryPreheatProcessed(ctx context.Context, store svc.ShowTimeStore, taskKey, reason string) error {
	fromStatus, consumeAttempts, err := store.MarkTaskProcessed(ctx, taskdef.TaskTypeRushInventoryPreheat, taskKey, time.Now())
	if err != nil {
		return err
	}
	logx.WithContext(ctx).Infow("delay_task_consume_state_transition",
		logx.Field("task_type", taskdef.TaskTypeRushInventoryPreheat),
		logx.Field("task_key", taskKey),
		logx.Field("from_status", fromStatus),
		logx.Field("to_status", 3),
		logx.Field("consume_attempts", consumeAttempts),
		logx.Field("reason", reason),
	)
	return nil
}

func markRushInventoryPreheatFailed(ctx context.Context, store svc.ShowTimeStore, taskKey string, consumeErr error) error {
	fromStatus, consumeAttempts, err := store.MarkTaskConsumeFailed(ctx, taskdef.TaskTypeRushInventoryPreheat, taskKey, time.Now(), consumeErr.Error())
	if err != nil {
		return err
	}
	logx.WithContext(ctx).Errorw("delay_task_consume_state_transition_failed",
		logx.Field("task_type", taskdef.TaskTypeRushInventoryPreheat),
		logx.Field("task_key", taskKey),
		logx.Field("from_status", fromStatus),
		logx.Field("to_status", 4),
		logx.Field("consume_attempts", consumeAttempts),
		logx.Field("error", consumeErr.Error()),
	)
	return consumeErr
}
