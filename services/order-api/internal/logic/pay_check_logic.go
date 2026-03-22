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

type PayCheckLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPayCheckLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayCheckLogic {
	return &PayCheckLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PayCheckLogic) PayCheck(req *types.PayCheckReq) (resp *types.PayCheckResp, err error) {
	userID, ok := xmiddleware.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, xerr.ErrUnauthorized.Error())
	}

	rpcResp, err := l.svcCtx.OrderRpc.PayCheck(l.ctx, &orderrpc.PayCheckReq{
		UserId:      userID,
		OrderNumber: req.OrderNumber,
	})
	if err != nil {
		return nil, err
	}

	return mapPayCheckResp(rpcResp), nil
}
