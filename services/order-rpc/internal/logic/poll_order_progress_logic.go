package logic

import (
	"context"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/rush"
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

	record, err := l.svcCtx.AttemptStore.Get(l.ctx, in.GetOrderNumber())
	if err != nil {
		return nil, mapOrderError(err)
	}
	if record.UserID != in.GetUserId() {
		return nil, mapOrderError(xerr.ErrOrderNotFound)
	}

	orderStatus, done, err := rush.MapAttemptRecordToPoll(record, time.Now())
	if err != nil {
		return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
	}

	return &pb.PollOrderProgressResp{
		OrderNumber: record.OrderNumber,
		OrderStatus: orderStatus,
		Done:        done,
	}, nil
}
