package logic

import (
	"context"

	"damai-go/jobs/order-close/internal/svc"
	"damai-go/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseExpiredOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCloseExpiredOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseExpiredOrdersLogic {
	return &CloseExpiredOrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CloseExpiredOrdersLogic) RunOnce() error {
	resp, err := l.svcCtx.OrderRpc.CloseExpiredOrders(l.ctx, &orderrpc.CloseExpiredOrdersReq{
		Limit: l.svcCtx.Config.BatchSize,
	})
	if err != nil {
		return err
	}

	l.Infof("order-close run finished, closedCount=%d", resp.GetClosedCount())
	return nil
}
