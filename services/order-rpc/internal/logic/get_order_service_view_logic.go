package logic

import (
	"context"
	"errors"

	"damai-go/services/order-rpc/internal/model"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetOrderServiceViewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetOrderServiceViewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderServiceViewLogic {
	return &GetOrderServiceViewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetOrderServiceViewLogic) GetOrderServiceView(in *pb.GetOrderServiceViewReq) (*pb.OrderServiceViewResp, error) {
	if err := validateUserOrderReq(in.GetUserId(), in.GetOrderNumber()); err != nil {
		return nil, err
	}

	order, err := findOwnedOrder(l.ctx, l.svcCtx, in.GetUserId(), in.GetOrderNumber())
	if err != nil {
		return nil, mapOrderError(err)
	}

	details, err := l.svcCtx.DOrderTicketUserModel.FindByOrderNumber(l.ctx, in.GetOrderNumber())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			details = nil
		} else {
			return nil, mapOrderError(err)
		}
	}

	payStatus, err := derivePayStatus(l.ctx, l.svcCtx, order)
	if err != nil {
		return nil, mapOrderError(err)
	}

	preview, err := previewRefundOrder(l.ctx, l.svcCtx, order)
	if err != nil {
		return nil, mapOrderError(err)
	}

	return mapOrderServiceView(order, payStatus, deriveTicketStatus(order, details), preview), nil
}
