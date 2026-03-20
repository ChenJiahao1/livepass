package logic

import (
	"context"
	"errors"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderLogic {
	return &GetOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetOrderLogic) GetOrder(in *pb.GetOrderReq) (*pb.OrderDetailInfo, error) {
	if err := validateUserOrderReq(in.GetUserId(), in.GetOrderNumber()); err != nil {
		return nil, err
	}

	order, err := l.svcCtx.DOrderModel.FindOneByOrderNumber(l.ctx, in.GetOrderNumber())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, mapOrderError(xerr.ErrOrderNotFound)
		}
		return nil, err
	}
	if order.UserId != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrOrderNotFound)
	}

	details, err := l.svcCtx.DOrderTicketUserModel.FindByOrderNumber(l.ctx, in.GetOrderNumber())
	if err != nil {
		return nil, err
	}

	return mapOrderDetail(order, details), nil
}
