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

type PollOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPollOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PollOrderLogic {
	return &PollOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PollOrderLogic) PollOrder(req *types.PollOrderReq) (resp *types.PollOrderResp, err error) {
	userID, ok := xmiddleware.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, xerr.ErrUnauthorized.Error())
	}

	rpcResp, err := l.svcCtx.OrderRpc.PollOrderProgress(l.ctx, &orderrpc.PollOrderProgressReq{
		UserId:      userID,
		OrderNumber: req.OrderNumber,
		ShowTimeId:  req.ShowTimeID,
	})
	if err != nil {
		return nil, err
	}

	return mapPollOrderResp(rpcResp), nil
}
