package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	return nil, status.Error(codes.Unimplemented, "verify attempt due is not implemented in task1 contract phase")
}
