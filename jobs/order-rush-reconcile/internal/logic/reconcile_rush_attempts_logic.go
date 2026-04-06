package logic

import (
	"context"

	"damai-go/jobs/order-rush-reconcile/internal/svc"
	"damai-go/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *ReconcileRushAttemptsLogic) RunOnce() error {
	resp, err := l.svcCtx.OrderRpc.ReconcileRushAttempts(l.ctx, &orderrpc.ReconcileRushAttemptsReq{
		Limit: l.svcCtx.Config.BatchSize,
	})
	if err != nil {
		return err
	}

	l.Infof(
		"order-rush-reconcile run finished, batchSize=%d reconciledCount=%d",
		l.svcCtx.Config.BatchSize,
		resp.GetReconciledCount(),
	)
	return nil
}
