// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"damai-go/pkg/xerr"
	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"damai-go/services/user-rpc/userrpc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	userID, err := requireCurrentUserID(l.ctx)
	if err != nil {
		return nil, err
	}

	ticketUsers, err := l.svcCtx.UserRpc.ListTicketUsers(l.ctx, &userrpc.ListTicketUsersReq{
		UserId: userID,
	})
	if err != nil {
		return nil, err
	}

	if !containsTicketUserID(ticketUsers, req.ID) {
		return nil, status.Error(codes.Unauthenticated, xerr.ErrUnauthorized.Error())
	}

	rpcResp, err := l.svcCtx.UserRpc.DeleteTicketUser(l.ctx, &userrpc.DeleteTicketUserReq{
		Id: req.ID,
	})
	if err != nil {
		return nil, err
	}

	return mapBoolResp(rpcResp), nil
}

func containsTicketUserID(resp *userrpc.ListTicketUsersResp, ticketUserID int64) bool {
	if resp == nil {
		return false
	}

	for _, item := range resp.List {
		if item != nil && item.Id == ticketUserID {
			return true
		}
	}

	return false
}
