package logic

import (
	"context"

	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteTicketUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteTicketUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteTicketUserLogic {
	return &DeleteTicketUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteTicketUserLogic) DeleteTicketUser(in *pb.DeleteTicketUserReq) (*pb.BoolResp, error) {
	return &pb.BoolResp{}, nil
}
