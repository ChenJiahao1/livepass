package logic

import (
	"context"

	"damai-go/jobs/order-migrate/internal/config"
	"damai-go/jobs/order-migrate/internal/svc"
	"damai-go/services/order-rpc/sharding"

	"github.com/zeromicro/go-zero/core/logx"
)

type BackfillOrdersResp struct {
	ProcessedCount int64
	LastOrderID    int64
}

type BackfillOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBackfillOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BackfillOrdersLogic {
	return &BackfillOrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BackfillOrdersLogic) BackfillOrders() (*BackfillOrdersResp, error) {
	targetSlots := resolveBackfillSlots(l.svcCtx.Config.Backfill.Slots, l.svcCtx.RouteMapConfig)
	updatedConfig, _, err := updateRouteEntries(
		l.svcCtx.RouteMapConfig,
		targetSlots,
		sharding.RouteStatusBackfilling,
		sharding.WriteModeDualWrite,
	)
	if err != nil {
		return nil, err
	}
	if err := l.svcCtx.SaveRouteMapConfig(updatedConfig); err != nil {
		return nil, err
	}

	checkpoint, err := loadBackfillCheckpoint(l.svcCtx.Config.Backfill.CheckpointFile)
	if err != nil {
		return nil, err
	}

	batchSize := l.svcCtx.Config.Backfill.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	orders, err := listLegacyOrdersAfter(l.ctx, l.svcCtx.LegacySqlConn, checkpoint.LastOrderID, batchSize)
	if err != nil {
		return nil, err
	}

	resp := &BackfillOrdersResp{}
	slotSet := make(map[int]struct{}, len(targetSlots))
	for _, logicSlot := range targetSlots {
		slotSet[logicSlot] = struct{}{}
	}
	initialCheckpoint := checkpoint.LastOrderID
	for _, order := range orders {
		checkpoint.LastOrderID = order.Id
		resp.LastOrderID = order.Id

		logicSlot, err := logicSlotForOrder(order)
		if err != nil {
			return nil, err
		}
		if _, ok := slotSet[logicSlot]; !ok {
			continue
		}
		route, err := l.svcCtx.RouteMap.RouteByLogicSlot(logicSlot)
		if err != nil {
			return nil, err
		}

		tickets, err := listOrderTicketsByNumber(l.ctx, l.svcCtx.LegacySqlConn, "d_order_ticket_user", order.OrderNumber)
		if err != nil {
			return nil, err
		}
		if err := upsertOrderBundle(l.ctx, l.svcCtx, route, order, tickets); err != nil {
			return nil, err
		}

		resp.ProcessedCount++
	}

	if checkpoint.LastOrderID != initialCheckpoint {
		if err := saveBackfillCheckpoint(l.svcCtx.Config.Backfill.CheckpointFile, checkpoint); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func resolveBackfillSlots(configuredSlots []int, routeMapConfig config.RouteMapConfig) []int {
	if len(configuredSlots) > 0 {
		return configuredSlots
	}

	slots := make([]int, 0, len(routeMapConfig.Entries))
	for _, entry := range routeMapConfig.Entries {
		slots = append(slots, entry.LogicSlot)
	}
	return slots
}
