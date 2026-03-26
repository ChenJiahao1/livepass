package logic

import (
	"context"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

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

	if l.svcCtx.Redis == nil {
		return &pb.GetOrderCacheResp{}, nil
	}

	cache, err := GetOrderCreateMarker(l.ctx, l.svcCtx.Redis, in.GetOrderNumber())
	if err != nil {
		return &pb.GetOrderCacheResp{}, nil
	}

	return &pb.GetOrderCacheResp{Cache: cache}, nil
}
