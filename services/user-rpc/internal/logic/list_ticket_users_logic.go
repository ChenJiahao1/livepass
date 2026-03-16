package logic

import (
	"context"

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
	// todo: add your logic here and delete this line

	return &pb.ListTicketUsersResp{}, nil
}
