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

type GetUserAndTicketUserListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserAndTicketUserListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserAndTicketUserListLogic {
	return &GetUserAndTicketUserListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserAndTicketUserListLogic) GetUserAndTicketUserList(req *types.GetUserAndTicketUserListReq) (resp *types.GetUserAndTicketUserListResp, err error) {
	userID, err := requireCurrentUserID(l.ctx)
	if err != nil {
		return nil, err
	}

	rpcResp, err := l.svcCtx.UserRpc.GetUserAndTicketUserList(l.ctx, &userrpc.GetUserAndTicketUserListReq{
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}

	if rpcResp == nil {
		return &types.GetUserAndTicketUserListResp{}, nil
	}

	return &types.GetUserAndTicketUserListResp{
		UserVo:           *mapUserVo(rpcResp.UserVo),
		TicketUserVoList: mapTicketUserVoList(rpcResp.TicketUserVoList),
	}, nil
}
