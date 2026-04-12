package logic

import (
	"context"
	"fmt"
	"time"

	"damai-go/jobs/order-close/internal/svc"
	orderrpc "damai-go/services/order-rpc/orderrpc"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	orderStatusUnpaid = 1

	orderDateTimeLayout = "2006-01-02 15:04:05"
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
	if l.svcCtx == nil || l.svcCtx.OutboxStore == nil {
		return fmt.Errorf("order-close outbox store is not configured")
	}
	if l.svcCtx.OrderRpc == nil {
		return fmt.Errorf("order-close order rpc is not configured")
	}

	items, err := l.svcCtx.OutboxStore.ListPendingOrderCreatedOutboxes(l.ctx, l.svcCtx.Config.BatchSize)
	if err != nil {
		return err
	}

	publishedCount := 0
	enqueuedCount := 0
	closedCount := 0
	for _, item := range items {
		detail, err := l.svcCtx.OrderRpc.GetOrder(l.ctx, &orderrpc.GetOrderReq{
			UserId:      item.UserID,
			OrderNumber: item.OrderNumber,
		})
		if err != nil {
			return err
		}

		switch {
		case detail.GetOrderStatus() != orderStatusUnpaid:
		default:
			expireAt, err := time.ParseInLocation(orderDateTimeLayout, detail.GetOrderExpireTime(), time.Local)
			if err != nil {
				return err
			}
			if !expireAt.After(time.Now()) {
				if _, err := l.svcCtx.OrderRpc.CloseExpiredOrder(l.ctx, &orderrpc.CloseExpiredOrderReq{
					OrderNumber: item.OrderNumber,
				}); err != nil {
					return err
				}
				closedCount++
				break
			}
			if l.svcCtx.AsyncCloseClient == nil {
				return fmt.Errorf("order-close async close client is not configured")
			}
			if err := l.svcCtx.AsyncCloseClient.EnqueueCloseTimeout(l.ctx, item.OrderNumber, expireAt); err != nil {
				return err
			}
			enqueuedCount++
		}

		if err := l.svcCtx.OutboxStore.MarkOutboxPublished(l.ctx, item.Ref, time.Now()); err != nil {
			return err
		}
		publishedCount++
	}

	l.Infof(
		"order-close run finished, scanned=%d published=%d enqueued=%d closed=%d",
		len(items),
		publishedCount,
		enqueuedCount,
		closedCount,
	)
	return nil
}
