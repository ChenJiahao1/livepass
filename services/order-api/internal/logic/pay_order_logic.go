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

type PayOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPayOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayOrderLogic {
	return &PayOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PayOrderLogic) PayOrder(req *types.PayOrderReq) (resp *types.PayOrderResp, err error) {
	userID, ok := xmiddleware.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, xerr.ErrUnauthorized.Error())
	}

	rpcResp, err := l.svcCtx.OrderRpc.PayOrder(l.ctx, &orderrpc.PayOrderReq{
		UserId:      userID,
		OrderNumber: req.OrderNumber,
		Subject:     req.Subject,
		Channel:     req.Channel,
	})
	if err != nil {
		return nil, err
	}

	return mapPayOrderResp(rpcResp), nil
}
