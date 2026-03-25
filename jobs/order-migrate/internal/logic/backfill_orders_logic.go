package logic

import (
	"context"

	"damai-go/jobs/order-migrate/internal/svc"

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
	for _, order := range orders {
		logicSlot, err := logicSlotForOrder(order)
		if err != nil {
			return nil, err
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

		checkpoint.LastOrderID = order.Id
		resp.ProcessedCount++
		resp.LastOrderID = order.Id
	}

	if resp.ProcessedCount > 0 {
		if err := saveBackfillCheckpoint(l.svcCtx.Config.Backfill.CheckpointFile, checkpoint); err != nil {
			return nil, err
		}
	}

	return resp, nil
}
