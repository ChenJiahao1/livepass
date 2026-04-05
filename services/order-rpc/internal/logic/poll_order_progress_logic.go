package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PollOrderProgressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPollOrderProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PollOrderProgressLogic {
	return &PollOrderProgressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PollOrderProgressLogic) PollOrderProgress(in *pb.PollOrderProgressReq) (*pb.PollOrderProgressResp, error) {
	// todo: add your logic here and delete this line

	return &pb.PollOrderProgressResp{}, nil
}
