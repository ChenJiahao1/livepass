// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteTicketUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteTicketUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTicketUserLogic {
	return &DeleteTicketUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteTicketUserLogic) DeleteTicketUser(req *types.DeleteTicketUserReq) (resp *types.BoolResp, err error) {
	rpcResp, err := l.svcCtx.UserRpc.DeleteTicketUser(l.ctx, &userrpc.DeleteTicketUserReq{
		Id: req.ID,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}
