package logic

import (
	"context"

	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddTicketUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddTicketUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddTicketUserLogic {
	return &AddTicketUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AddTicketUserLogic) AddTicketUser(in *pb.AddTicketUserReq) (*pb.BoolResp, error) {
	// todo: add your logic here and delete this line

	return &pb.BoolResp{}, nil
}
