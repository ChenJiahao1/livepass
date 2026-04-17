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

type CreatePurchaseTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePurchaseTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePurchaseTokenLogic {
	return &CreatePurchaseTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePurchaseTokenLogic) CreatePurchaseToken(req *types.CreatePurchaseTokenReq) (resp *types.CreatePurchaseTokenResp, err error) {
	userID, ok := xmiddleware.UserIDFromContext(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, xerr.ErrUnauthorized.Error())
	}

	rpcResp, err := l.svcCtx.OrderRpc.CreatePurchaseToken(l.ctx, &orderrpc.CreatePurchaseTokenReq{
		UserId:           userID,
		ShowTimeId:       req.ShowTimeID,
		TicketCategoryId: req.TicketCategoryID,
		TicketUserIds:    req.TicketUserIds,
		DistributionMode: req.DistributionMode,
		TakeTicketMode:   req.TakeTicketMode,
	})
	if err != nil {
		return nil, err
	}

	return mapCreatePurchaseTokenResp(rpcResp), nil
}
