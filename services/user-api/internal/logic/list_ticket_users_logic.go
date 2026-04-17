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

type ListTicketUsersLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTicketUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketUsersLogic {
	return &ListTicketUsersLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListTicketUsersLogic) ListTicketUsers(req *types.ListTicketUsersReq) (resp *types.TicketUserListResp, err error) {
	userID, err := requireCurrentUserID(l.ctx)
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.UserRpc.ListTicketUsers(l.ctx, &userrpc.ListTicketUsersReq{
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}

	if rpcResp == nil {
		return &types.TicketUserListResp{}, nil
	}

	return &types.TicketUserListResp{
		List: mapTicketUserVoList(rpcResp.List),
	}, nil
}
