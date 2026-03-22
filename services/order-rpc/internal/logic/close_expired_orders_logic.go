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

	orders, err := l.svcCtx.DOrderModel.FindExpiredUnpaid(l.ctx, time.Now(), limit)
	if err != nil {
		return nil, err
	}

	resp := &pb.CloseExpiredOrdersResp{}
	for _, order := range orders {
		changed, err := cancelOrderWithLock(l.ctx, l.svcCtx, order.OrderNumber, 0, false, "order_expired_close")
		if err != nil {
			return nil, mapOrderError(err)
		}
		if changed {
			resp.ClosedCount++
		}
	}

	return resp, nil
}
