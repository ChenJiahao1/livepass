package logic

import (
	"context"

	"damai-go/services/user-rpc/internal/svc"
	"damai-go/services/user-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyCaptchaLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyCaptchaLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyCaptchaLogic {
	return &VerifyCaptchaLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyCaptchaLogic) VerifyCaptcha(in *pb.VerifyCaptchaReq) (*pb.BoolResp, error) {
	// todo: add your logic here and delete this line

	return &pb.BoolResp{}, nil
}
