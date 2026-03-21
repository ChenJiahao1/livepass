// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmiddleware"
	"damai-go/services/order-api/internal/svc"
	"damai-go/services/order-api/internal/types"
	"damai-go/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RefundOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefundOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundOrderLogic {
	return &RefundOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefundOrderLogic) RefundOrder(req *types.RefundOrderReq) (resp *types.RefundOrderResp, err error) {
	userID, ok := xmiddleware.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, xerr.ErrUnauthorized.Error())
	}

	rpcResp, err := l.svcCtx.OrderRpc.RefundOrder(l.ctx, &orderrpc.RefundOrderReq{
		UserId:      userID,
		OrderNumber: req.OrderNumber,
		Reason:      req.Reason,
	})
	if err != nil {
		return nil, err
	}

	return mapRefundOrderResp(rpcResp), nil
}
