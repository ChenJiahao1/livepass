// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"livepass/services/order-api/internal/svc"
	"livepass/services/order-api/internal/types"
	"livepass/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetOrderCacheLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetOrderCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderCacheLogic {
	return &GetOrderCacheLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetOrderCacheLogic) GetOrderCache(req *types.OrderCacheReq) (resp *types.OrderCacheResp, err error) {
	rpcResp, err := l.svcCtx.OrderRpc.GetOrderCache(l.ctx, &orderrpc.GetOrderCacheReq{
		OrderNumber: req.OrderNumber,
	})
	if err != nil {
		return nil, err
	}

	return &types.OrderCacheResp{Cache: rpcResp.GetCache()}, nil
}
