package logic

import (
	"context"

	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMobileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateMobileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMobileLogic {
	return &UpdateMobileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateMobileLogic) UpdateMobile(in *pb.UpdateMobileReq) (*pb.BoolResp, error) {
	return &pb.BoolResp{}, nil
}
