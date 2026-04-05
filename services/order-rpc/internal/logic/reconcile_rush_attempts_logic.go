package logic

import (
	"context"
	"errors"
	"time"

	"damai-go/pkg/xerr"
	"damai-go/services/order-rpc/internal/svc"
	"damai-go/services/order-rpc/pb"

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

func (l *ReconcileRushAttemptsLogic) ReconcileRushAttempts(in *pb.ReconcileRushAttemptsReq) (*pb.ReconcileRushAttemptsResp, error) {
	if l.svcCtx == nil || l.svcCtx.AttemptStore == nil || l.svcCtx.OrderRepository == nil {
		return nil, mapOrderError(xerr.ErrInternal)
	}

	limit := in.GetLimit()
	if limit <= 0 {
		limit = defaultRushReconcileLimit
	}

	orderNumbers, err := l.svcCtx.AttemptStore.ScanOrderNumbers(l.ctx, limit)
	if err != nil {
		return nil, mapOrderError(err)
	}

	resp := &pb.ReconcileRushAttemptsResp{}
	now := time.Now()
	for _, orderNumber := range orderNumbers {
		record, err := l.svcCtx.AttemptStore.Get(l.ctx, orderNumber)
		if err != nil {
			if errors.Is(err, xerr.ErrOrderNotFound) {
				continue
			}
			return nil, mapOrderError(err)
		}
		if !shouldReconcileRushAttempt(record, now) {
			continue
		}

		changed, err := advanceRushAttemptProjection(l.ctx, l.svcCtx, record, now)
		if err != nil {
			return nil, mapOrderError(err)
		}
		if changed {
			resp.ReconciledCount++
		}
		if resp.ReconciledCount >= limit {
			break
		}
	}

	return resp, nil
}
