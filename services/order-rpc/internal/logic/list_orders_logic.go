package logic

import (
	"context"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOrdersLogic {
	return &ListOrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListOrdersLogic) ListOrders(in *pb.ListOrdersReq) (*pb.ListOrdersResp, error) {
	if in.GetUserId() <= 0 {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}

	pageNumber := in.GetPageNumber()
	if pageNumber <= 0 {
		pageNumber = 1
	}
	pageSize := in.GetPageSize()
	if pageSize <= 0 {
		pageSize = 10
	}

	orders, total, err := l.svcCtx.OrderRepository.FindOrderPageByUser(l.ctx, in.GetUserId(), in.GetOrderStatus(), pageNumber, pageSize)
	if err != nil {
		return nil, err
	}

	resp := &pb.ListOrdersResp{
		PageNum:   pageNumber,
		PageSize:  pageSize,
		TotalSize: total,
	}
	if len(orders) == 0 {
		return resp, nil
	}

	resp.List = make([]*pb.OrderListInfo, 0, len(orders))
	for _, order := range orders {
		resp.List = append(resp.List, mapOrderSummary(order))
	}

	return resp, nil
}
