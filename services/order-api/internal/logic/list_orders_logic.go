// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"livepass/pkg/xerr"
	"livepass/pkg/xmiddleware"
	"livepass/services/order-api/internal/svc"
	"livepass/services/order-api/internal/types"
	"livepass/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListOrdersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListOrdersLogic {
	return &ListOrdersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListOrdersLogic) ListOrders(req *types.ListOrdersReq) (resp *types.ListOrdersResp, err error) {
	userID, ok := xmiddleware.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, xerr.ErrUnauthorized.Error())
	}

	pageNumber := req.PageNumber
	if pageNumber <= 0 {
		pageNumber = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	rpcResp, err := l.svcCtx.OrderRpc.ListOrders(l.ctx, &orderrpc.ListOrdersReq{
		UserId:      userID,
		PageNumber:  pageNumber,
		PageSize:    pageSize,
		OrderStatus: req.OrderStatus,
	})
	if err != nil {
		return nil, err
	}

	return mapListOrdersResp(rpcResp), nil
}
