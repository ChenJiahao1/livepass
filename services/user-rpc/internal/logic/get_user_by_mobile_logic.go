package logic

import (
	"context"

	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserByMobileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserByMobileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByMobileLogic {
	return &GetUserByMobileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserByMobileLogic) GetUserByMobile(in *pb.GetUserByMobileReq) (*pb.UserInfo, error) {
	return &pb.UserInfo{}, nil
}
