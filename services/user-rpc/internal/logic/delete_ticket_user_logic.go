package logic

import (
	"context"

	"livepass/services/user-rpc/internal/svc"
	"livepass/services/user-rpc/pb"

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
	if err := l.svcCtx.DTicketUserModel.Delete(l.ctx, in.Id); err != nil {
		return nil, err
	}
	return &pb.BoolResp{Success: true}, nil
}
