package logic

import (
	"context"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CancelOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelOrderLogic {
	return &CancelOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelOrderLogic) CancelOrder(in *pb.CancelOrderReq) (*pb.BoolResp, error) {
	if err := validateUserOrderReq(in.GetUserId(), in.GetOrderNumber()); err != nil {
		return nil, err
	}

	_, err := cancelOrderWithLock(l.ctx, l.svcCtx, in.GetOrderNumber(), in.GetUserId(), true, "order_cancel")
	if err != nil {
		if err == xerr.ErrInvalidParam {
			return nil, mapOrderError(err)
		}
		return nil, mapOrderError(err)
	}

	return &pb.BoolResp{Success: true}, nil
}
