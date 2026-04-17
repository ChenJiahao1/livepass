package logic

import (
	"context"
	"errors"
	"time"

	"livepass/pkg/xerr"
	"livepass/services/order-rpc/internal/model"
	"livepass/services/order-rpc/internal/rush"
	"livepass/services/order-rpc/internal/svc"
)

type orderProgressProjection struct {
	OrderNumber int64
	UserID      int64
	OrderStatus int64
	Done        bool
	ReasonCode  string
}

func projectOrderProgress(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64, now time.Time) (*orderProgressProjection, error) {
	if orderNumber <= 0 {
		return nil, xerr.ErrInvalidParam
	}
	if svcCtx == nil || svcCtx.AttemptStore == nil {
		return nil, xerr.ErrInternal
	}
	if now.IsZero() {
		now = time.Now()
	}

	record, err := svcCtx.AttemptStore.Get(ctx, orderNumber)
	switch {
	case err == nil:
		status, done, mapErr := rush.MapAttemptRecordToPoll(record, now)
		if mapErr != nil {
			return nil, xerr.ErrInternal
		}

		reasonCode := ""
		if done && status == rush.PollOrderStatusFailed {
			reasonCode = record.ReasonCode
		}
		return &orderProgressProjection{
			OrderNumber: record.OrderNumber,
			UserID:      record.UserID,
			OrderStatus: status,
			Done:        done,
			ReasonCode:  reasonCode,
		}, nil
	case !errors.Is(err, xerr.ErrOrderNotFound):
		return nil, err
	default:
		return projectOrderProgressFromDB(ctx, svcCtx, orderNumber)
	}
}

func projectOrderProgressFromDB(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64) (*orderProgressProjection, error) {
	if svcCtx == nil || svcCtx.OrderRepository == nil {
		return &orderProgressProjection{
			OrderNumber: orderNumber,
			OrderStatus: rush.PollOrderStatusFailed,
			Done:        true,
		}, nil
	}

	order, err := svcCtx.OrderRepository.FindOrderByNumber(ctx, orderNumber)
	switch {
	case err == nil && order != nil:
		return &orderProgressProjection{
			OrderNumber: orderNumber,
			UserID:      order.UserId,
			OrderStatus: rush.PollOrderStatusSuccess,
			Done:        true,
		}, nil
	case err == nil:
		return &orderProgressProjection{
			OrderNumber: orderNumber,
			OrderStatus: rush.PollOrderStatusFailed,
			Done:        true,
		}, nil
	case errors.Is(err, model.ErrNotFound):
		return &orderProgressProjection{
			OrderNumber: orderNumber,
			OrderStatus: rush.PollOrderStatusFailed,
			Done:        true,
		}, nil
	default:
		return nil, err
	}
}
