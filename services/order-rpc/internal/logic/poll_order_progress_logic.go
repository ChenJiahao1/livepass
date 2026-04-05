package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	return nil, status.Error(codes.Unimplemented, "poll order progress is not implemented in task1 contract phase")
}
