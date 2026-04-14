package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"damai-go/services/program-rpc/internal/model"
	"damai-go/services/program-rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type rushInventoryPreheatTxClient interface {
	EnqueueWithConn(ctx context.Context, conn sqlx.SqlConn, programID int64, expectedOpenTime time.Time) error
}

func scheduleRushInventoryPreheat(ctx context.Context, svcCtx *svc.ServiceContext, programModel model.DProgramModel, showTimeModel model.DProgramShowTimeModel, conn sqlx.SqlConn, program *model.DProgram) error {
	if program == nil || !program.RushSaleOpenTime.Valid || svcCtx.RushInventoryPreheatClient == nil {
		return nil
	}
	if programModel == nil {
		programModel = svcCtx.DProgramModel
	}
	if showTimeModel == nil {
		showTimeModel = svcCtx.DProgramShowTimeModel
	}
	if programModel == nil || showTimeModel == nil {
		return nil
	}

	showTimes, err := showTimeModel.FindByProgramIds(ctx, []int64{program.Id})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil
		}
		return err
	}
	if len(showTimes) == 0 {
		return nil
	}

	previousStatus := program.InventoryPreheatStatus
	now := time.Now()
	if previousStatus != 1 {
		program.InventoryPreheatStatus = 1
		program.EditTime = now
		if err := programModel.Update(ctx, program); err != nil {
			return err
		}
	}

	if txClient, ok := svcCtx.RushInventoryPreheatClient.(rushInventoryPreheatTxClient); ok && conn != nil {
		err = txClient.EnqueueWithConn(ctx, conn, program.Id, program.RushSaleOpenTime.Time)
	} else {
		err = svcCtx.RushInventoryPreheatClient.Enqueue(ctx, program.Id, program.RushSaleOpenTime.Time)
	}
	if err != nil {
		if conn != nil || previousStatus == 1 {
			return err
		}

		program.InventoryPreheatStatus = previousStatus
		program.EditTime = time.Now()
		if rollbackErr := programModel.Update(ctx, program); rollbackErr != nil {
			return fmt.Errorf("enqueue rush inventory preheat: %w; rollback inventory_preheat_status: %v", err, rollbackErr)
		}
		return err
	}

	return nil
}
