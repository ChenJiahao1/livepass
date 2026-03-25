package logic

import (
	"context"
	"time"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseExpiredOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseExpiredOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseExpiredOrdersLogic {
	return &CloseExpiredOrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CloseExpiredOrdersLogic) CloseExpiredOrders(in *pb.CloseExpiredOrdersReq) (*pb.CloseExpiredOrdersResp, error) {
	limit := in.GetLimit()
	if limit <= 0 {
		limit = 100
	}

	resp := &pb.CloseExpiredOrdersResp{}
	now := time.Now()
	logicSlotStart, logicSlotCount := normalizeCloseExpiredOrderSlotWindow(in)
	remaining := limit
	for logicSlot := logicSlotStart; logicSlot < logicSlotStart+logicSlotCount && remaining > 0; logicSlot++ {
		orders, err := l.svcCtx.OrderRepository.FindExpiredUnpaidBySlot(l.ctx, int(logicSlot), now, remaining)
		if err != nil {
			return nil, err
		}

		for _, order := range orders {
			changed, err := cancelOrderWithLock(l.ctx, l.svcCtx, order.OrderNumber, 0, false, "order_expired_close")
			if err != nil {
				return nil, mapOrderError(err)
			}
			if changed {
				resp.ClosedCount++
			}
		}
		remaining -= int64(len(orders))
	}

	return resp, nil
}

func normalizeCloseExpiredOrderSlotWindow(in *pb.CloseExpiredOrdersReq) (int64, int64) {
	if in == nil {
		return 0, 1
	}

	start := in.GetLogicSlotStart()
	if start < 0 {
		start = 0
	}

	count := in.GetLogicSlotCount()
	if count <= 0 {
		count = 1
	}

	return start, count
}
