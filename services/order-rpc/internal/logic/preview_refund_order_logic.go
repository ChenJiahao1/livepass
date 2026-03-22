package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PreviewRefundOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPreviewRefundOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PreviewRefundOrderLogic {
	return &PreviewRefundOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PreviewRefundOrderLogic) PreviewRefundOrder(in *pb.PreviewRefundOrderReq) (*pb.PreviewRefundOrderResp, error) {
	if err := validateUserOrderReq(in.GetUserId(), in.GetOrderNumber()); err != nil {
		return nil, err
	}

	order, err := findOwnedOrder(l.ctx, l.svcCtx, in.GetUserId(), in.GetOrderNumber())
	if err != nil {
		return nil, mapOrderError(err)
	}

	resp, err := previewRefundOrder(l.ctx, l.svcCtx, order)
	if err != nil {
		return nil, mapOrderError(err)
	}

	return resp, nil
}
