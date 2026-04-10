package logic

import (
	"context"
	"time"

	"damai-go/pkg/xerr"
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
	if in == nil || in.GetUserId() <= 0 || in.GetOrderNumber() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}
	if l.svcCtx == nil || l.svcCtx.AttemptStore == nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	projection, err := projectOrderProgress(l.ctx, l.svcCtx, in.GetOrderNumber(), time.Now())
	if err != nil {
		return nil, mapOrderError(err)
	}
	if projection.UserID > 0 && projection.UserID != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrOrderNotFound)
	}

	return &pb.PollOrderProgressResp{
		OrderNumber: projection.OrderNumber,
		OrderStatus: projection.OrderStatus,
		Done:        projection.Done,
		ReasonCode:  projection.ReasonCode,
	}, nil
}
