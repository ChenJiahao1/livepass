// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"

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
	// todo: add your logic here and delete this line

	return
}
