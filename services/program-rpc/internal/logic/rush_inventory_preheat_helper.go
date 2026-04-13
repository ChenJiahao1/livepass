package logic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type rushInventoryPreheatTxClient interface {
	EnqueueWithConn(ctx context.Context, conn sqlx.SqlConn, showTimeID int64, expectedOpenTime time.Time) error
}

func scheduleRushInventoryPreheat(ctx context.Context, svcCtx *svc.ServiceContext, showTimeModel model.DProgramShowTimeModel, conn sqlx.SqlConn, showTime *model.DProgramShowTime) error {
	if showTime == nil || !showTime.RushSaleOpenTime.Valid || svcCtx.RushInventoryPreheatClient == nil {
		return nil
	}
	if showTimeModel == nil {
		showTimeModel = svcCtx.DProgramShowTimeModel
	}

	previousStatus := showTime.InventoryPreheatStatus
	now := time.Now()
	if previousStatus != 1 {
		showTime.InventoryPreheatStatus = 1
		showTime.EditTime = sql.NullTime{Time: now, Valid: true}
		if err := showTimeModel.Update(ctx, showTime); err != nil {
			return err
		}
	}

	var err error
	if txClient, ok := svcCtx.RushInventoryPreheatClient.(rushInventoryPreheatTxClient); ok && conn != nil {
		err = txClient.EnqueueWithConn(ctx, conn, showTime.Id, showTime.RushSaleOpenTime.Time)
	} else {
		err = svcCtx.RushInventoryPreheatClient.Enqueue(ctx, showTime.Id, showTime.RushSaleOpenTime.Time)
	}
	if err != nil {
		if conn != nil || previousStatus == 1 {
			return err
		}

		showTime.InventoryPreheatStatus = previousStatus
		showTime.EditTime = sql.NullTime{Time: time.Now(), Valid: true}
		if rollbackErr := showTimeModel.Update(ctx, showTime); rollbackErr != nil {
			return fmt.Errorf("enqueue rush inventory preheat: %w; rollback inventory_preheat_status: %v", err, rollbackErr)
		}
		return err
	}

	return nil
}
