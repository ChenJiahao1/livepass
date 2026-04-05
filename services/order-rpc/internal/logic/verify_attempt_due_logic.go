package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type VerifyAttemptDueLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewVerifyAttemptDueLogic(ctx context.Context, svcCtx *svc.ServiceContext) *VerifyAttemptDueLogic {
	return &VerifyAttemptDueLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *VerifyAttemptDueLogic) VerifyAttemptDue(in *pb.VerifyAttemptDueReq) (*pb.BoolResp, error) {
	// todo: add your logic here and delete this line

	return &pb.BoolResp{}, nil
}
