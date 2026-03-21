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

type PayCheckLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPayCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayCheckLogic {
	return &PayCheckLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PayCheckLogic) PayCheck(in *pb.PayCheckReq) (*pb.PayCheckResp, error) {
	if err := validateUserOrderReq(in.GetUserId(), in.GetOrderNumber()); err != nil {
		return nil, err
	}

	order, err := l.svcCtx.DOrderModel.FindOneByOrderNumber(l.ctx, in.GetOrderNumber())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, mapOrderError(xerr.ErrOrderNotFound)
		}
		return nil, mapOrderError(err)
	}
	if order.UserId != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrOrderNotFound)
	}
	if order.OrderStatus != orderStatusPaid && order.OrderStatus != orderStatusRefunded {
		return mapPayCheckResp(order, nil), nil
	}

	payBill, err := mustGetPayBillForOrder(l.ctx, l.svcCtx, order.OrderNumber)
	if err != nil {
		return nil, mapOrderError(err)
	}

	return mapPayCheckResp(order, payBill), nil
}
