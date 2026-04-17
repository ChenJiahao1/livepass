package logic

import (
	"context"
	"strconv"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
	"livepass/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetOrderCacheLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetOrderCacheLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderCacheLogic {
	return &GetOrderCacheLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetOrderCacheLogic) GetOrderCache(in *pb.GetOrderCacheReq) (*pb.GetOrderCacheResp, error) {
	if in.GetOrderNumber() <= 0 {
		return nil, status.Error(codes.InvalidArgument, xerr.ErrInvalidParam.Error())
	}

	if l.svcCtx == nil || l.svcCtx.AttemptStore == nil {
		return &pb.GetOrderCacheResp{}, nil
	}

	projection, err := projectOrderProgress(l.ctx, l.svcCtx, in.GetOrderNumber(), time.Now())
	if err != nil {
		return &pb.GetOrderCacheResp{}, nil
	}
	if projection.OrderStatus != rush.PollOrderStatusProcessing || projection.Done {
		return &pb.GetOrderCacheResp{}, nil
	}

	return &pb.GetOrderCacheResp{Cache: strconv.FormatInt(in.GetOrderNumber(), 10)}, nil
}
