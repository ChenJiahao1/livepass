// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"livepass/services/user-api/internal/svc"
	"livepass/services/user-api/internal/types"
	"livepass/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddTicketUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddTicketUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddTicketUserLogic {
	return &AddTicketUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddTicketUserLogic) AddTicketUser(req *types.AddTicketUserReq) (resp *types.BoolResp, err error) {
	userID, err := requireCurrentUserID(l.ctx)
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.UserRpc.AddTicketUser(l.ctx, &userrpc.AddTicketUserReq{
		UserId:   userID,
		RelName:  req.RelName,
		IdType:   req.IdType,
		IdNumber: req.IdNumber,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
