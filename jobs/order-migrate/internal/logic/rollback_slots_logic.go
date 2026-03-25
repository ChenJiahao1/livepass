package logic

import (
	"context"

	"damai-go/jobs/order-migrate/internal/svc"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/logx"
)

type RollbackSlotsResp struct {
	UpdatedSlots int64
}

type RollbackSlotsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRollbackSlotsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RollbackSlotsLogic {
	return &RollbackSlotsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RollbackSlotsLogic) RollbackSlots() (*RollbackSlotsResp, error) {
	updatedConfig, updatedCount, err := updateRouteEntries(
		l.svcCtx.RouteMapConfig,
		l.svcCtx.Config.Rollback.Slots,
		sharding.RouteStatusRollback,
		sharding.WriteModeLegacyPrimary,
	)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.SaveRouteMapConfig(updatedConfig); err != nil {
		return nil, err
	}

	return &RollbackSlotsResp{UpdatedSlots: updatedCount}, nil
}
