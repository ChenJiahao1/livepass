package logic

import (
	"context"

	"damai-go/jobs/order-migrate/internal/svc"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyOrdersResp struct {
	VerifiedSlots  int64
	ComparedOrders int64
	LastOrderID    int64
	Completed      bool
}

type VerifyOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyOrdersLogic {
	return &VerifyOrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyOrdersLogic) VerifyOrders() (*VerifyOrdersResp, error) {
	routes := make([]sharding.Route, 0, len(l.svcCtx.Config.Verify.Slots))
	for _, logicSlot := range l.svcCtx.Config.Verify.Slots {
		route, err := l.svcCtx.RouteMap.RouteByLogicSlot(logicSlot)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}

	checkpoint, err := loadVerifyCheckpoint(l.svcCtx.Config.Verify.CheckpointFile)
	if err != nil {
		return nil, err
	}

	legacyData, lastOrderID, completed, err := loadLegacyVerifySlotData(
		l.ctx,
		l.svcCtx.LegacySqlConn,
		l.svcCtx.Config.Verify.Slots,
		checkpoint.LastOrderID,
		l.svcCtx.Config.Verify.BatchSize,
	)
	if err != nil {
		return nil, err
	}
	shardData, err := loadShardVerifySlotData(l.ctx, l.svcCtx.ShardSqlConns, routes, checkpoint.LastOrderID, lastOrderID)
	if err != nil {
		return nil, err
	}

	resp := &VerifyOrdersResp{
		LastOrderID: lastOrderID,
		Completed:   completed,
	}
	for _, logicSlot := range l.svcCtx.Config.Verify.Slots {
		legacySlotData := legacyData[logicSlot]
		shardSlotData := shardData[logicSlot]
		if err := compareAggregates(
			logicSlot,
			buildVerifyAggregate(legacySlotData.orders),
			buildVerifyAggregate(shardSlotData.orders),
		); err != nil {
			return nil, err
		}
		if err := compareOrderNumberSets(legacySlotData.orders, shardSlotData.orders); err != nil {
			return nil, err
		}
		if err := compareOrderSamples(l.svcCtx.Config.Verify.SampleSize, legacySlotData.orders, shardSlotData.orders); err != nil {
			return nil, err
		}
		if err := compareUserListSamples(l.svcCtx.Config.Verify.SampleSize, legacySlotData.orders, shardSlotData.orders); err != nil {
			return nil, err
		}
		if err := compareTicketSnapshots(legacySlotData.tickets, shardSlotData.tickets); err != nil {
			return nil, err
		}
		resp.ComparedOrders += int64(len(legacySlotData.orders))
	}

	if lastOrderID != checkpoint.LastOrderID {
		checkpoint.LastOrderID = lastOrderID
		if err := saveVerifyCheckpoint(l.svcCtx.Config.Verify.CheckpointFile, checkpoint); err != nil {
			return nil, err
		}
	}
	if !completed {
		return resp, nil
	}

	updatedConfig, _, err := updateRouteEntries(
		l.svcCtx.RouteMapConfig,
		l.svcCtx.Config.Verify.Slots,
		sharding.RouteStatusVerifying,
		sharding.WriteModeDualWrite,
	)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.SaveRouteMapConfig(updatedConfig); err != nil {
		return nil, err
	}
	resp.VerifiedSlots = int64(len(l.svcCtx.Config.Verify.Slots))

	return resp, nil
}
