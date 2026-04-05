package logic

import (
	"context"

	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ReconcileRushAttemptsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReconcileRushAttemptsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReconcileRushAttemptsLogic {
	return &ReconcileRushAttemptsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReconcileRushAttemptsLogic) ReconcileRushAttempts(in *pb.ReconcileRushAttemptsReq) (*pb.ReconcileRushAttemptsResp, error) {
	return nil, status.Error(codes.Unimplemented, "reconcile rush attempts is not implemented in task1 contract phase")
}
