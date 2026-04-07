// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/services/order-api/internal/svc"
	"damai-go/services/order-api/internal/types"
	"damai-go/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AccountOrderCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAccountOrderCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AccountOrderCountLogic {
	return &AccountOrderCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AccountOrderCountLogic) AccountOrderCount(req *types.AccountOrderCountReq) (resp *types.AccountOrderCountResp, err error) {
	rpcResp, err := l.svcCtx.OrderRpc.CountActiveTicketsByUserShowTime(l.ctx, &orderrpc.CountActiveTicketsByUserShowTimeReq{
		UserId:     req.UserID,
		ShowTimeId: req.ShowTimeID,
	})
	if err != nil {
		return nil, err
	}

	return &types.AccountOrderCountResp{Count: rpcResp.GetActiveTicketCount()}, nil
}
