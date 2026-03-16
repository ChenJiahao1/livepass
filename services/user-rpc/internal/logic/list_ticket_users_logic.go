package logic

import (
	"context"
	"errors"

	"damai-go/services/user-rpc/internal/model"
	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTicketUsersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListTicketUsersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTicketUsersLogic {
	return &ListTicketUsersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListTicketUsersLogic) ListTicketUsers(in *pb.ListTicketUsersReq) (*pb.ListTicketUsersResp, error) {
	list, err := l.svcCtx.DTicketUserModel.FindByUserId(l.ctx, in.UserId)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, err
	}
	resp := &pb.ListTicketUsersResp{}
	for _, item := range list {
		resp.List = append(resp.List, buildTicketUserInfo(item))
	}
	return resp, nil
}
