package logic

import (
	"context"

	"damai-go/jobs/order-close/internal/config"
	"damai-go/jobs/order-close/internal/svc"
	"damai-go/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseExpiredOrdersLogic struct {
	ctx          context.Context
	svcCtx       *svc.ServiceContext
	nextScanSlot int64
	logx.Logger
}

func NewCloseExpiredOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseExpiredOrdersLogic {
	initialSlot := int64(0)
	if svcCtx != nil {
		initialSlot = normalizeCheckpointSlot(svcCtx.Config)
	}

	return &CloseExpiredOrdersLogic{
		ctx:          ctx,
		svcCtx:       svcCtx,
		nextScanSlot: initialSlot,
		Logger:       logx.WithContext(ctx),
	}
}

func (l *CloseExpiredOrdersLogic) RunOnce() error {
	logicSlotStart, logicSlotCount, nextSlot := l.nextScanWindow()
	resp, err := l.svcCtx.OrderRpc.CloseExpiredOrders(l.ctx, &orderrpc.CloseExpiredOrdersReq{
		Limit:          l.svcCtx.Config.BatchSize,
		LogicSlotStart: logicSlotStart,
		LogicSlotCount: logicSlotCount,
	})
	if err != nil {
		return err
	}
	l.nextScanSlot = nextSlot

	l.Infof(
		"order-close run finished, logicSlotStart=%d logicSlotCount=%d nextSlot=%d closedCount=%d",
		logicSlotStart,
		logicSlotCount,
		nextSlot,
		resp.GetClosedCount(),
	)
	return nil
}

func (l *CloseExpiredOrdersLogic) nextScanWindow() (int64, int64, int64) {
	cfg := l.svcCtx.Config
	slotStart := cfg.ScanSlotStart
	slotEnd := cfg.ScanSlotEnd
	if slotEnd < slotStart {
		slotEnd = slotStart
	}

	batchSize := cfg.ScanSlotBatchSize
	if batchSize <= 0 {
		batchSize = 1
	}

	current := l.nextScanSlot
	if current < slotStart || current > slotEnd {
		current = slotStart
	}

	count := batchSize
	if current+count-1 > slotEnd {
		count = slotEnd - current + 1
	}
	if count <= 0 {
		count = 1
	}

	nextSlot := current + count
	if nextSlot > slotEnd {
		nextSlot = slotStart
	}

	return current, count, nextSlot
}

func normalizeCheckpointSlot(cfg config.Config) int64 {
	if cfg.ScanSlotEnd < cfg.ScanSlotStart {
		cfg.ScanSlotEnd = cfg.ScanSlotStart
	}
	if cfg.CheckpointSlot < cfg.ScanSlotStart || cfg.CheckpointSlot > cfg.ScanSlotEnd {
		return cfg.ScanSlotStart
	}
	return cfg.CheckpointSlot
}
