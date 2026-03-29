package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseExpiredOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseExpiredOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseExpiredOrderLogic {
	return &CloseExpiredOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CloseExpiredOrderLogic) CloseExpiredOrder(in *pb.CloseExpiredOrderReq) (*pb.BoolResp, error) {
	if in == nil || in.GetOrderNumber() <= 0 {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}

	order, err := loadOrderSnapshot(l.ctx, l.svcCtx, in.GetOrderNumber())
	if err != nil {
		if errors.Is(err, xerr.ErrOrderNotFound) {
			return &pb.BoolResp{Success: true}, nil
		}
		return nil, mapOrderError(err)
	}
	if order.OrderStatus != orderStatusUnpaid {
		return &pb.BoolResp{Success: true}, nil
	}
	if order.OrderExpireTime.After(time.Now()) {
		return &pb.BoolResp{Success: true}, nil
	}

	_, err = cancelOrderWithLock(l.ctx, l.svcCtx, in.GetOrderNumber(), 0, false, "order_expired_close")
	if err != nil {
		if errors.Is(err, xerr.ErrOrderStatusInvalid) {
			return &pb.BoolResp{Success: true}, nil
		}
		return nil, mapOrderError(err)
	}

	return &pb.BoolResp{Success: true}, nil
}
