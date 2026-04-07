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
	if in == nil || in.GetOrderNumber() <= 0 {
		return nil, mapOrderError(xerr.ErrInvalidParam)
	}
	if l.svcCtx == nil || l.svcCtx.AttemptStore == nil || l.svcCtx.OrderRepository == nil {
		return nil, mapOrderError(xerr.ErrInternal)
	}

	record, err := l.svcCtx.AttemptStore.Get(l.ctx, in.GetOrderNumber())
	if err != nil {
		if errors.Is(err, xerr.ErrOrderNotFound) {
			return &pb.BoolResp{Success: true}, nil
		}
		return nil, mapOrderError(err)
	}

	now := time.Now()
	if dueAtUnix := in.GetDueAtUnix(); dueAtUnix > 0 && now.Before(time.Unix(dueAtUnix, 0)) {
		return &pb.BoolResp{Success: true}, nil
	}

	_, err = verifyRushAttemptProjection(l.ctx, l.svcCtx, record, now)
	if err != nil {
		return nil, mapOrderError(err)
	}

	return &pb.BoolResp{Success: true}, nil
}
