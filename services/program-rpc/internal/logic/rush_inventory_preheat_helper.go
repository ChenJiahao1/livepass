package logic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"
)

func scheduleRushInventoryPreheat(ctx context.Context, svcCtx *svc.ServiceContext, showTime *model.DProgramShowTime) error {
	if showTime == nil || !showTime.RushSaleOpenTime.Valid || svcCtx.RushInventoryPreheatClient == nil {
		return nil
	}

	previousStatus := showTime.InventoryPreheatStatus
	now := time.Now()
	if previousStatus != 1 {
		showTime.InventoryPreheatStatus = 1
		showTime.EditTime = sql.NullTime{Time: now, Valid: true}
		if err := svcCtx.DProgramShowTimeModel.Update(ctx, showTime); err != nil {
			return err
		}
	}

	if err := svcCtx.RushInventoryPreheatClient.Enqueue(ctx, showTime.Id, showTime.RushSaleOpenTime.Time); err != nil {
		if previousStatus == 1 {
			return err
		}

		showTime.InventoryPreheatStatus = previousStatus
		showTime.EditTime = sql.NullTime{Time: time.Now(), Valid: true}
		if rollbackErr := svcCtx.DProgramShowTimeModel.Update(ctx, showTime); rollbackErr != nil {
			return fmt.Errorf("enqueue rush inventory preheat: %w; rollback inventory_preheat_status: %v", err, rollbackErr)
		}
		return err
	}

	return nil
}
