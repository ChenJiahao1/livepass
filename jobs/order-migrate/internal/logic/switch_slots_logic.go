package logic

import (
	"context"
	"fmt"

	"damai-go/jobs/order-migrate/internal/config"
	"damai-go/jobs/order-migrate/internal/svc"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/logx"
)

type SwitchSlotsResp struct {
	UpdatedSlots int64
}

type SwitchSlotsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSwitchSlotsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SwitchSlotsLogic {
	return &SwitchSlotsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SwitchSlotsLogic) SwitchSlots() (*SwitchSlotsResp, error) {
	updatedConfig, updatedCount, err := updateRouteEntries(
		l.svcCtx.RouteMapConfig,
		l.svcCtx.Config.Switch.Slots,
		sharding.RouteStatusPrimaryNew,
		sharding.WriteModeShardPrimary,
	)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.SaveRouteMapConfig(updatedConfig); err != nil {
		return nil, err
	}

	return &SwitchSlotsResp{UpdatedSlots: updatedCount}, nil
}

func updateRouteEntries(routeMapConfig config.RouteMapConfig, slots []int, targetStatus, targetWriteMode string) (config.RouteMapConfig, int64, error) {
	slotSet := make(map[int]struct{}, len(slots))
	for _, slot := range slots {
		slotSet[slot] = struct{}{}
	}

	var updatedCount int64
	for idx := range routeMapConfig.Entries {
		entry := &routeMapConfig.Entries[idx]
		if _, ok := slotSet[entry.LogicSlot]; !ok {
			continue
		}
		if err := sharding.ValidateRouteStatusTransition(entry.Status, targetStatus); err != nil {
			return config.RouteMapConfig{}, 0, err
		}
		entry.Status = targetStatus
		entry.WriteMode = targetWriteMode
		updatedCount++
	}
	if updatedCount == 0 {
		return config.RouteMapConfig{}, 0, fmt.Errorf("no route entries updated")
	}

	return routeMapConfig, updatedCount, nil
}
