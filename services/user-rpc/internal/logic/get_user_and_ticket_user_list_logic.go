package logic

import (
	"context"

	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserAndTicketUserListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserAndTicketUserListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserAndTicketUserListLogic {
	return &GetUserAndTicketUserListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserAndTicketUserListLogic) GetUserAndTicketUserList(in *pb.GetUserAndTicketUserListReq) (*pb.GetUserAndTicketUserListResp, error) {
	return &pb.GetUserAndTicketUserListResp{}, nil
}
